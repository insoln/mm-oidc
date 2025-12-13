package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const httpClientTimeout = 10 * time.Second

// Plugin wires the Mattermost lifecycle hooks to the OIDC-specific implementation.
type Plugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *Configuration
	metadata          *OIDCMetadata

	httpClient *http.Client
	router     http.Handler

	oidcProvider *oidc.Provider
}

// NewPlugin exposes the constructor so main() stays minimal.
func NewPlugin() *Plugin {
	return &Plugin{}
}

// OnActivate prepares long-lived dependencies like the outbound HTTP client.
func (p *Plugin) OnActivate() error {
	p.httpClient = &http.Client{
		Timeout: httpClientTimeout,
	}

	return p.OnConfigurationChange()
}

// OnConfigurationChange reloads and validates the latest settings from the System Console.
func (p *Plugin) OnConfigurationChange() error {
	var config Configuration
	if err := p.API.LoadPluginConfiguration(&config); err != nil {
		p.API.LogError("failed to load configuration", "error", err.Error())
		return err
	}

	config.SetDefaults()
	if err := config.Validate(); err != nil {
		p.API.LogError("invalid configuration", "error", err.Error())
		return err
	}

	if err := p.refreshMetadata(&config); err != nil {
		p.API.LogError("failed to refresh OIDC metadata", "error", err.Error())
		return err
	}

	if err := p.initOIDCProvider(&config); err != nil {
		p.API.LogError("failed to initialize OIDC provider", "error", err.Error())
		return err
	}

	p.setConfiguration(&config)
	p.resetRouter()
	return nil
}

// ServeHTTP is the shared entrypoint for Mattermost's inter-plugin HTTP traffic.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	if handler := p.getRouter(); handler != nil {
		handler.ServeHTTP(w, r)
		return
	}

	http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
}

func (p *Plugin) getConfiguration() *Configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &Configuration{}
	}

	return p.configuration.Clone()
}

func (p *Plugin) setConfiguration(config *Configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()
	p.configuration = config.Clone()
}

func (p *Plugin) getProvider() *oidc.Provider {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()
	return p.oidcProvider
}

func (p *Plugin) setProvider(provider *oidc.Provider) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()
	p.oidcProvider = provider
}

func (p *Plugin) getMetadata() *OIDCMetadata {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()
	if p.metadata == nil {
		return nil
	}
	copy := *p.metadata
	return &copy
}

func (p *Plugin) getRouter() http.Handler {
	p.configurationLock.RLock()
	handler := p.router
	p.configurationLock.RUnlock()

	if handler != nil {
		return handler
	}

	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if p.router == nil {
		mux := http.NewServeMux()
		mux.HandleFunc("/", p.handleLanding)
		mux.HandleFunc("/health", p.handleHealth)
		mux.HandleFunc("/login", p.handleLogin)
		mux.HandleFunc("/callback", p.handleCallback)
		mux.HandleFunc("/logout", p.handleLogout)
		p.router = mux
	}

	return p.router
}

func (p *Plugin) resetRouter() {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()
	p.router = nil
}

func (p *Plugin) handleHealth(w http.ResponseWriter, _ *http.Request) {
	cfg := p.getConfiguration()
	status := "ready"
	if p.getMetadata() == nil || p.getProvider() == nil {
		status = "error"
	}
	payload := map[string]interface{}{
		"status":       status,
		"issuer_url":   cfg.IssuerURL,
		"redirect_url": cfg.RedirectURL,
	}

	p.respondJSON(w, payload, http.StatusOK)
}

func (p *Plugin) handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	cfg := p.getConfiguration()
	issuer := htmlEscape(cfg.IssuerURL)
	redirect := htmlEscape(cfg.RedirectURL)
	metadataReady := p.getMetadata() != nil
	providerReady := p.getProvider() != nil
	ready := metadataReady && providerReady

	ctaMarkup := `<div class="actions">
		<button class="primary" onclick="window.location.href='./login'">Start Login</button>
		<button class="secondary" onclick="window.location.reload()">Refresh Status</button>
	</div>`
	statusVariant := "ready"
	statusLabel := "Ready"
	statusCopy := "The plugin is ready to start the OIDC login flow."

	switch {
	case !ready:
		statusVariant = "error"
		statusLabel = "Unavailable"
		statusCopy = "We couldn't verify the plugin health. Review the configuration or logs, then reload this page."
		ctaMarkup = `<p class="notice">Resolve the configuration or connectivity issues before attempting to sign in.</p>
		<div class="actions">
			<button class="secondary" onclick="window.location.reload()">Retry Health Check</button>
		</div>`
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8" />
	<title>Mattermost OIDC Bridge</title>
	<style>
		body { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; margin: 0; padding: 2rem; background: #0f172a; color: #f1f5f9; }
		.card { max-width: 640px; margin: 0 auto; background: rgba(15,23,42,0.85); border-radius: 16px; padding: 2rem; box-shadow: 0 15px 60px rgba(15,23,42,0.4); }
		h1 { margin-top: 0; font-size: 1.8rem; }
		p { line-height: 1.5; }
		.meta { font-size: 0.9rem; color: #cbd5f5; margin-top: 1.5rem; }
		.actions { margin-top: 1.5rem; display: flex; gap: 0.75rem; flex-wrap: wrap; }
		.primary, .secondary { border: none; border-radius: 999px; padding: 0.85rem 1.6rem; font-weight: 600; cursor: pointer; }
		.primary { background: #38bdf8; color: #0f172a; box-shadow: 0 10px 25px rgba(56,189,248,0.35); }
		.primary:hover { transform: translateY(-1px); box-shadow: 0 16px 30px rgba(56,189,248,0.4); }
		.secondary { background: transparent; color: #f1f5f9; border: 1px solid rgba(241,245,249,0.3); }
		.secondary:hover { transform: translateY(-1px); }
		.notice { margin-top: 1rem; padding: 0.85rem 1rem; border-left: 3px solid rgba(56,189,248,0.6); background: rgba(15,23,42,0.6); border-radius: 12px; color: #cbd5f5; }
		.badge { display: inline-flex; align-items: center; padding: 0.2rem 0.8rem; border-radius: 999px; font-size: 0.85rem; font-weight: 600; }
		.badge[data-variant='ready'] { background: rgba(34,197,94,0.2); color: #4ade80; }
		.badge[data-variant='error'] { background: rgba(248,113,113,0.2); color: #f87171; }
	</style>
</head>
<body>
	<main class="card">
		<h1>Mattermost OIDC Bridge</h1>
		<p>%s</p>
		%s
		<div class="meta">
			<div><strong>Status:</strong> <span class="badge" data-variant="%s">%s</span></div>
			<div><strong>Issuer:</strong> %s</div>
			<div><strong>Redirect URL:</strong> %s</div>
			<div><strong>Plugin ID:</strong> %s</div>
		</div>
	</main>
</body>
</html>`, statusCopy, ctaMarkup, statusVariant, statusLabel, issuer, redirect, htmlEscape(pluginID))
}

func (p *Plugin) writeFriendlyError(w http.ResponseWriter, status int, title, message string) {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(title) == "" {
		title = http.StatusText(status)
	}
	if strings.TrimSpace(message) == "" {
		message = "An unexpected error occurred. Please try again."
	}

	liveTitle := htmlEscape(title)
	liveMessage := htmlEscape(message)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8" />
	<title>%s · Mattermost OIDC</title>
	<style>
		body { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; margin: 0; padding: 2rem; background: #111827; color: #f9fafb; }
		.card { max-width: 560px; margin: 0 auto; background: rgba(15,23,42,0.9); border-radius: 18px; padding: 2rem; box-shadow: 0 20px 65px rgba(15,23,42,0.55); }
		h1 { margin-top: 0; font-size: 1.75rem; }
		p { line-height: 1.5; }
		.actions { margin-top: 1.5rem; display: flex; gap: 0.75rem; flex-wrap: wrap; }
		a.primary { display: inline-flex; align-items: center; justify-content: center; padding: 0.85rem 1.6rem; border-radius: 999px; font-weight: 600; background: #f97316; color: #111827; text-decoration: none; }
		a.primary:hover { opacity: 0.9; }
		a.secondary { display: inline-flex; align-items: center; justify-content: center; padding: 0.8rem 1.25rem; border-radius: 999px; font-weight: 500; color: #f97316; border: 1px solid rgba(249,115,22,0.4); text-decoration: none; }
		a.secondary:hover { border-color: rgba(249,115,22,0.8); }
	</style>
</head>
<body>
	<main class="card">
		<h1>%s</h1>
		<p>%s</p>
		<div class="actions">
			<a class="primary" href="./login">Try again</a>
			<a class="secondary" href="./">Back to plugin home</a>
		</div>
	</main>
</body>
</html>`, liveTitle, liveTitle, liveMessage)
}

func friendlyProvisioningError(err error) (int, string, string, bool) {
	if err == nil {
		return 0, "", "", false
	}

	var appErr *model.AppError
	if errors.As(err, &appErr) && isLastAdminDemotionError(appErr) {
		message := "Mattermost cannot remove system administrator access from the final admin account. Promote another system administrator or keep this account as an admin before trying again."
		return http.StatusForbidden, "Cannot remove last System Admin", message, true
	}

	return 0, "", "", false
}

func isLastAdminDemotionError(appErr *model.AppError) bool {
	if appErr == nil {
		return false
	}

	ids := []string{
		"api.user.demote_last_admin.app_error",
		"api.user.demote_last_admin",
	}
	for _, candidate := range ids {
		if strings.EqualFold(strings.TrimSpace(appErr.Id), candidate) {
			return true
		}
	}

	haystack := []string{appErr.Message, appErr.DetailedError, appErr.Error()}
	for _, text := range haystack {
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "cannot demote last system admin") {
			return true
		}
	}

	return false
}

func (p *Plugin) handleLogin(w http.ResponseWriter, r *http.Request) {
	cfg := p.getConfiguration()
	metadata := p.getMetadata()
	if metadata == nil {
		p.writeFriendlyError(w, http.StatusServiceUnavailable, "Identity provider unavailable", "We can't reach the configured OIDC metadata right now. Please try again in a moment.")
		return
	}

	state, err := generateRandomString(stateBytes)
	if err != nil {
		p.API.LogError("failed to generate state", "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to start login", "We hit an unexpected error while preparing the login flow. Please try again.")
		return
	}

	nonce, err := generateRandomString(nonceBytes)
	if err != nil {
		p.API.LogError("failed to generate nonce", "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to start login", "We hit an unexpected error while preparing the login flow. Please try again.")
		return
	}

	codeVerifier, err := generateRandomString(pkceVerifierBytes)
	if err != nil {
		p.API.LogError("failed to generate code verifier", "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to start login", "We hit an unexpected error while preparing the login flow. Please try again.")
		return
	}

	codeChallenge := pkceChallenge(codeVerifier)
	authorizeURL, err := buildAuthorizeURL(metadata.AuthorizationEndpoint, cfg, state, nonce, codeChallenge)
	if err != nil {
		p.API.LogError("failed to build authorize URL", "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to start login", "We couldn't build a valid authorization request. Please try again.")
		return
	}

	session := &authSession{
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		CreatedAt:    time.Now().Unix(),
	}
	if err := p.saveAuthSession(state, session); err != nil {
		p.API.LogError("failed to persist auth session", "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to start login", "We couldn't store the temporary login session. Please try again.")
		return
	}

	p.API.LogDebug("redirecting to OIDC provider", "state", state)
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (p *Plugin) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		p.writeFriendlyError(w, http.StatusBadRequest, "Invalid login response", "We could not validate the parameters returned from the identity provider. Please start a new login from Mattermost.")
		return
	}

	cfg := p.getConfiguration()
	metadata := p.getMetadata()
	provider := p.getProvider()
	if metadata == nil || provider == nil {
		p.writeFriendlyError(w, http.StatusServiceUnavailable, "Identity provider unavailable", "We can't reach the configured OIDC metadata right now. Please try again in a moment.")
		return
	}

	session, err := p.consumeAuthSession(state)
	if err != nil {
		p.API.LogError("failed to load auth session", "state", state, "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to validate login", "We couldn't validate your login session. Please try again.")
		return
	}

	if session == nil {
		p.writeFriendlyError(w, http.StatusGone, "Login link expired", "Your login session has expired. Please launch the login flow again.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), tokenExchangeTimeout)
	defer cancel()

	tokens, err := p.exchangeCode(ctx, cfg, metadata, code, session.CodeVerifier)
	if err != nil {
		p.API.LogError("token exchange failed", "state", state, "error", err.Error())
		p.writeFriendlyError(w, http.StatusBadGateway, "Unable to complete login", "We couldn't exchange the authorization code with the identity provider. Please try again.")
		return
	}

	idTokenClaims, err := p.verifyIDToken(ctx, provider, cfg, tokens.IDToken, session.Nonce)
	if err != nil {
		p.API.LogError("id_token verification failed", "state", state, "error", err.Error())
		p.writeFriendlyError(w, http.StatusUnauthorized, "Unable to verify identity", "We could not verify the identity information from the provider. Please try again.")
		return
	}

	profile := buildUserProfile(idTokenClaims, cfg.ClientID)
	p.logClaimsSnapshot(idTokenClaims, cfg.ClientID, profile.SystemAdmin)
	user, err := p.provisionUser(profile)
	if err != nil {
		p.API.LogError("failed to provision user", "state", state, "error", err.Error())
		if status, title, message, handled := friendlyProvisioningError(err); handled {
			p.writeFriendlyError(w, status, title, message)
		} else {
			p.writeFriendlyError(w, http.StatusInternalServerError, "We couldn't finish signing you in", "Mattermost was unable to create or update your account. Please try again or contact your system administrator.")
		}
		return
	}

	createdSession, err := p.completeLogin(w, r, user, cfg)
	if err != nil {
		p.API.LogError("failed to complete login", "state", state, "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to finish signing you in", "We couldn't establish a Mattermost session. Please try again.")
		return
	}

	if err := p.persistSessionTokens(createdSession, tokens, cfg); err != nil {
		p.API.LogWarn("failed to persist session tokens", "state", state, "error", err.Error())
	}

	p.API.LogDebug("authentication successful", "sub", profile.Subject, "user_id", user.Id)
}

func (p *Plugin) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	cfg := p.getConfiguration()
	metadata := p.getMetadata()
	sessionToken := sessionTokenFromRequest(r)
	secure := requestIsSecure(r, cfg)
	landing := pluginLandingURL(cfg)

	var record *sessionTokenRecord
	var err error
	if sessionToken != "" {
		record, err = p.loadSessionTokenRecord(sessionToken, cfg)
		if err != nil {
			p.API.LogWarn("unable to load session tokens for logout", "error", err.Error())
		}
		if delErr := p.deleteSessionTokenRecord(sessionToken); delErr != nil {
			p.API.LogWarn("unable to delete session tokens", "error", delErr.Error())
		}
	}

	if record != nil {
		if record.SessionID != "" {
			if appErr := p.API.RevokeSession(record.SessionID); appErr != nil {
				p.API.LogWarn("unable to revoke Mattermost session", "session_id", record.SessionID, "error", appErr.Error())
			}
		}
	}

	clearSessionCookies(w, secure)

	if metadata != nil && strings.TrimSpace(metadata.EndSessionEndpoint) != "" && record != nil && strings.TrimSpace(record.IDToken) != "" {
		logoutURL, buildErr := buildLogoutURL(metadata.EndSessionEndpoint, record.IDToken, landing)
		if buildErr != nil {
			p.API.LogWarn("unable to construct logout redirect", "error", buildErr.Error())
		} else {
			http.Redirect(w, r, logoutURL, http.StatusFound)
			return
		}
	}

	http.Redirect(w, r, landing, http.StatusFound)
}

func (p *Plugin) respondJSON(w http.ResponseWriter, payload interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		p.API.LogError("failed to encode JSON response", "error", err.Error())
	}
}

func (p *Plugin) logClaimsSnapshot(claims *idTokenClaims, clientID string, isSystemAdmin bool) {
	if claims == nil {
		p.API.LogDebug("id_token claims missing", "client_id", clientID)
		return
	}

	snapshot := map[string]interface{}{
		"sub":                claims.Subject,
		"email":              claims.Email,
		"preferred_username": claims.PreferredUsername,
		"given_name":         claims.GivenName,
		"family_name":        claims.FamilyName,
		"resource_access":    flattenResourceAccess(claims.ResourceAccess),
		"client_id":          clientID,
	}

	if resourceAccess, ok := snapshot["resource_access"].(map[string][]string); ok {
		snapshot["client_roles"] = resourceAccess[clientID]
	}

	serialized, err := json.Marshal(snapshot)
	if err != nil {
		p.API.LogDebug("failed to serialize claim snapshot", "error", err.Error())
		return
	}

	p.API.LogDebug("id_token claims snapshot", "payload", string(serialized), "system_admin", isSystemAdmin)
}

func flattenResourceAccess(access map[string]clientRoleMapping) map[string][]string {
	if len(access) == 0 {
		return nil
	}

	result := make(map[string][]string, len(access))
	for client, mapping := range access {
		if len(mapping.Roles) == 0 {
			continue
		}
		result[client] = append([]string(nil), mapping.Roles...)
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func (p *Plugin) completeLogin(w http.ResponseWriter, r *http.Request, user *model.User, cfg *Configuration) (*model.Session, error) {
	session := &model.Session{UserId: user.Id, Roles: strings.TrimSpace(user.Roles)}
	if session.Roles == "" {
		session.Roles = model.SystemUserRoleId
	}
	session.PreSave()
	csrfToken := session.GenerateCSRF()
	session.Id = ""

	created, appErr := p.API.CreateSession(session)
	if appErr != nil {
		return nil, fmt.Errorf("create session: %w", appErr)
	}

	if csrfToken == "" {
		csrfToken = created.GetCSRF()
	}

	if err := issueSessionCookies(w, r, created, csrfToken, cfg); err != nil {
		return nil, err
	}

	redirectTarget := postLoginRedirect(cfg, p.API.GetConfig())
	http.Redirect(w, r, redirectTarget, http.StatusFound)
	return created, nil
}

func issueSessionCookies(w http.ResponseWriter, r *http.Request, session *model.Session, csrfToken string, cfg *Configuration) error {
	if session == nil || session.Token == "" {
		return fmt.Errorf("invalid session")
	}

	secure := requestIsSecure(r, cfg)
	sameSite := http.SameSiteLaxMode

	expires, maxAge := sessionCookieLifetime(session)

	tokenCookie := &http.Cookie{
		Name:     model.SessionCookieToken,
		Value:    session.Token,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: sameSite,
	}
	if maxAge > 0 {
		tokenCookie.MaxAge = maxAge
	}
	if !expires.IsZero() {
		tokenCookie.Expires = expires
	}
	http.SetCookie(w, tokenCookie)

	userCookie := &http.Cookie{
		Name:     model.SessionCookieUser,
		Value:    session.UserId,
		Path:     "/",
		Secure:   secure,
		HttpOnly: false,
		SameSite: sameSite,
	}
	if maxAge > 0 {
		userCookie.MaxAge = maxAge
	}
	if !expires.IsZero() {
		userCookie.Expires = expires
	}
	http.SetCookie(w, userCookie)

	csrfValue := csrfToken
	if csrfValue == "" {
		csrfValue = session.GetCSRF()
	}
	if csrfValue != "" {
		csrfCookie := &http.Cookie{
			Name:     model.SessionCookieCsrf,
			Value:    csrfValue,
			Path:     "/",
			Secure:   secure,
			HttpOnly: false,
			SameSite: sameSite,
		}
		if maxAge > 0 {
			csrfCookie.MaxAge = maxAge
		}
		if !expires.IsZero() {
			csrfCookie.Expires = expires
		}
		http.SetCookie(w, csrfCookie)
	}

	return nil
}

func sessionCookieLifetime(session *model.Session) (time.Time, int) {
	if session == nil || session.ExpiresAt == 0 {
		return time.Time{}, 0
	}

	expires := time.UnixMilli(session.ExpiresAt)
	remaining := time.Until(expires)
	if remaining <= 0 {
		return time.Time{}, 0
	}

	return expires, int(remaining.Seconds())
}

func postLoginRedirect(cfg *Configuration, mmCfg *model.Config) string {
	if mmCfg != nil && mmCfg.ServiceSettings.SiteURL != nil {
		if site := strings.TrimSpace(*mmCfg.ServiceSettings.SiteURL); site != "" {
			return site
		}
	}

	if cfg != nil && strings.TrimSpace(cfg.RedirectURL) != "" {
		if parsed, err := url.Parse(cfg.RedirectURL); err == nil {
			parsed.Path = "/"
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return parsed.String()
		}
	}

	return "/"
}

func pluginLandingURL(cfg *Configuration) string {
	if cfg == nil || strings.TrimSpace(cfg.RedirectURL) == "" {
		return fmt.Sprintf("/plugins/%s", pluginID)
	}

	parsed, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return fmt.Sprintf("/plugins/%s", pluginID)
	}
	parsed.Path = fmt.Sprintf("/plugins/%s", pluginID)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func buildLogoutURL(base, idToken, postLogout string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("end_session endpoint missing")
	}

	endpoint, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse end_session endpoint: %w", err)
	}

	q := endpoint.Query()
	if strings.TrimSpace(idToken) != "" {
		q.Set("id_token_hint", idToken)
	}
	if strings.TrimSpace(postLogout) != "" {
		q.Set("post_logout_redirect_uri", postLogout)
	}
	if state, err := generateRandomString(16); err == nil {
		q.Set("state", state)
	}
	endpoint.RawQuery = q.Encode()
	return endpoint.String(), nil
}

func clearSessionCookies(w http.ResponseWriter, secure bool) {
	expired := time.Unix(0, 0)
	for _, name := range []string{model.SessionCookieToken, model.SessionCookieUser, model.SessionCookieCsrf} {
		cookie := &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Secure:   secure,
			HttpOnly: name != model.SessionCookieUser,
			SameSite: http.SameSiteLaxMode,
			Expires:  expired,
			MaxAge:   -1,
		}
		http.SetCookie(w, cookie)
	}
}

func sessionTokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if cookie, err := r.Cookie(model.SessionCookieToken); err == nil {
		if value := strings.TrimSpace(cookie.Value); value != "" {
			return value
		}
	}
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		candidate := strings.TrimSpace(authz[7:])
		if candidate != "" {
			return candidate
		}
	}
	if header := strings.TrimSpace(r.Header.Get(model.HeaderToken)); header != "" {
		return header
	}
	return ""
}

func requestIsSecure(r *http.Request, cfg *Configuration) bool {
	if r != nil {
		if r.TLS != nil {
			return true
		}
		if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
			return true
		}
	}

	if cfg != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(cfg.RedirectURL)), "https://") {
		return true
	}

	return false
}

func htmlEscape(value string) string {
	return template.HTMLEscapeString(value)
}
