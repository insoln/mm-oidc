package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	httpClientTimeout  = 10 * time.Second
	redirectHintCookie = "MMOIDC_REDIRECT"
)

// Plugin wires the Mattermost lifecycle hooks to the OIDC-specific implementation.
type Plugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *Configuration
	metadata          *OIDCMetadata

	httpClient     *http.Client
	httpClientOnce sync.Once
	router         http.Handler

	oidcProvider *oidc.Provider
}

// NewPlugin exposes the constructor so main() stays minimal.
func NewPlugin() *Plugin {
	return &Plugin{}
}

// OnActivate prepares long-lived dependencies like the outbound HTTP client.
func (p *Plugin) OnActivate() error {
	p.ensureHTTPClient()
	return p.OnConfigurationChange()
}

// OnConfigurationChange reloads and validates the latest settings from the System Console.
func (p *Plugin) OnConfigurationChange() error {
	p.ensureHTTPClient()
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

func (p *Plugin) ensureHTTPClient() {
	p.httpClientOnce.Do(func() {
		p.httpClient = &http.Client{Timeout: httpClientTimeout}
	})
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
		mux.HandleFunc("/complete", p.handleMobileComplete)
		mux.HandleFunc("/complete/", p.handleMobileComplete)
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
		:root { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; }
		body { margin: 0; padding: 2rem; background: #0f172a; color: #f1f5f9; }
		.card { max-width: 720px; margin: 0 auto; background: rgba(15,23,42,0.85); border-radius: 18px; padding: 2.25rem; box-shadow: 0 15px 60px rgba(15,23,42,0.4); }
		h1 { margin-top: 0; font-size: 1.9rem; }
		h2 { margin: 0; font-size: 1.4rem; }
		p { line-height: 1.5; }
		.meta { font-size: 0.95rem; color: #cbd5f5; margin-top: 1.5rem; }
		.meta strong { display: inline-block; min-width: 120px; }
		.actions { margin-top: 1.5rem; display: flex; gap: 0.75rem; flex-wrap: wrap; }
		.primary, .secondary { border: none; border-radius: 999px; padding: 0.9rem 1.8rem; font-weight: 600; cursor: pointer; transition: transform 120ms ease, box-shadow 120ms ease; }
		.primary { background: #38bdf8; color: #0f172a; box-shadow: 0 12px 30px rgba(56,189,248,0.35); }
		.primary:hover { transform: translateY(-1px); box-shadow: 0 18px 35px rgba(56,189,248,0.4); }
		.secondary { background: transparent; color: #f1f5f9; border: 1px solid rgba(241,245,249,0.3); }
		.secondary:hover { transform: translateY(-1px); }
		.notice { margin-top: 1rem; padding: 0.85rem 1rem; border-left: 3px solid rgba(56,189,248,0.6); background: rgba(15,23,42,0.6); border-radius: 12px; color: #cbd5f5; }
		.badge { display: inline-flex; align-items: center; padding: 0.25rem 0.85rem; border-radius: 999px; font-size: 0.85rem; font-weight: 600; }
		.badge[data-variant='ready'] { background: rgba(34,197,94,0.2); color: #4ade80; }
		.badge[data-variant='error'] { background: rgba(248,113,113,0.2); color: #f87171; }
		.diag-section { margin-top: 2rem; padding: 2rem; border-radius: 20px; background: rgba(15,23,42,0.75); border: 1px solid rgba(148,163,184,0.25); display: flex; flex-direction: column; gap: 1.2rem; }
		.diag-header { display: flex; flex-wrap: wrap; gap: 1rem; justify-content: space-between; }
		.diag-actions { display: flex; flex-wrap: wrap; gap: 0.75rem; }
		.diag-button { border: 1px solid rgba(241,245,249,0.4); background: transparent; color: #f1f5f9; border-radius: 12px; padding: 0.65rem 1.1rem; font-weight: 600; cursor: pointer; transition: border-color 120ms ease, transform 120ms ease; }
		.diag-button:hover { border-color: #38bdf8; transform: translateY(-1px); }
		.diag-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.85rem; }
		.diag-item { background: rgba(15,23,42,0.6); border-radius: 14px; border: 1px solid rgba(148,163,184,0.2); padding: 0.9rem; }
		.diag-label { font-size: 0.75rem; letter-spacing: 0.08em; color: #94a3b8; text-transform: uppercase; }
		.diag-value { margin-top: 0.35rem; word-break: break-word; font-size: 0.95rem; }
		.diag-pre { background: rgba(2,6,23,0.9); border-radius: 14px; padding: 1rem; border: 1px solid rgba(148,163,184,0.2); max-height: 360px; overflow: auto; font-size: 0.85rem; }
		.kicker { letter-spacing: 0.2em; text-transform: uppercase; font-size: 0.75rem; color: #cbd5f5; margin: 0; }
		@media (max-width: 600px) {
			.primary, .secondary { width: 100%%; text-align: center; }
			.diag-actions { width: 100%%; }
			.diag-button { flex: 1 1 100%%; text-align: center; }
		}
	</style>
</head>
<body>
	<main class="card">
		<h1>Mattermost OIDC Bridge</h1>
		<p>%[1]s</p>
		%[2]s
		<div class="meta">
			<div><strong>Status:</strong> <span class="badge" data-variant="%[3]s">%[4]s</span></div>
			<div><strong>Issuer:</strong> %[5]s</div>
			<div><strong>Redirect URL:</strong> %[6]s</div>
			<div><strong>Plugin ID:</strong> %[7]s</div>
		</div>
		<section class="diag-section" aria-live="polite">
			<div class="diag-header">
				<div>
					<p class="kicker">Deep diagnostics</p>
					<h2>Login context snapshot</h2>
					<p>Use this snapshot when the desktop app lands here instead of the Mattermost content you expected.</p>
				</div>
				<div class="diag-actions">
					<button id="diag-refresh" class="diag-button" type="button">Refresh snapshot</button>
					<button id="diag-copy" class="diag-button" type="button">Copy JSON</button>
				</div>
			</div>
			<div id="diag-summary" class="diag-grid"></div>
			<pre id="diag-json" class="diag-pre">Collecting…</pre>
		</section>
	</main>
	<script>
	(function() {
		const diagJson = document.getElementById('diag-json');
		const diagSummary = document.getElementById('diag-summary');
		const refreshBtn = document.getElementById('diag-refresh');
		const copyBtn = document.getElementById('diag-copy');
		const cookieHintName = 'MMOIDC_REDIRECT';
		const pluginReady = %[8]t;
		const statusVariant = '%[3]s';
		const pluginId = '%[7]s';
		const issuer = '%[5]s';
		const redirectURL = '%[6]s';

		function parseCookies() {
			return document.cookie
				.split(';')
				.map((chunk) => chunk.trim())
				.filter(Boolean)
				.map((entry) => entry.split('=')[0]);
		}

		function collectDiagnostics() {
			const snapshot = {
				generated_at: new Date().toISOString(),
				plugin_ready: pluginReady,
				status_variant: statusVariant,
				plugin_id: pluginId,
				issuer_url: issuer,
				redirect_url: redirectURL,
			};

			if (typeof window !== 'undefined') {
				snapshot.location_href = window.location?.href ?? '';
				snapshot.location_pathname = window.location?.pathname ?? '';
				snapshot.location_search = window.location?.search ?? '';
				snapshot.location_hash = window.location?.hash ?? '';
				snapshot.location_origin = window.location?.origin ?? '';
				snapshot.query_params = window.location?.search ? Object.fromEntries(new URLSearchParams(window.location.search)) : {};
				snapshot.navigator_user_agent = window.navigator?.userAgent ?? '';
				snapshot.navigator_language = window.navigator?.language ?? '';
				snapshot.navigator_online = window.navigator?.onLine ?? false;
				snapshot.navigator_platform = window.navigator?.platform ?? '';
				snapshot.hardware_concurrency = window.navigator?.hardwareConcurrency;
				if (window.navigator && 'deviceMemory' in window.navigator) {
					snapshot.device_memory_gb = window.navigator.deviceMemory;
				}
				snapshot.viewport = {
					inner_width: window.innerWidth,
					inner_height: window.innerHeight,
					outer_width: window.outerWidth,
					outer_height: window.outerHeight,
				};
				snapshot.screen = window.screen ? { width: window.screen.width, height: window.screen.height, pixel_ratio: window.devicePixelRatio ?? 1 } : null;
				if (typeof Intl !== 'undefined' && Intl.DateTimeFormat) {
					snapshot.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
				}
				try {
					snapshot.local_storage_keys = window.localStorage ? Object.keys(window.localStorage) : [];
				} catch (error) {
					snapshot.local_storage_error = error?.message ?? 'unavailable';
				}
				try {
					snapshot.session_storage_keys = window.sessionStorage ? Object.keys(window.sessionStorage) : [];
				} catch (error) {
					snapshot.session_storage_error = error?.message ?? 'unavailable';
				}
			}

			if (typeof document !== 'undefined') {
				snapshot.document_referrer = document.referrer ?? '';
				snapshot.visibility_state = document.visibilityState ?? '';
				snapshot.document_has_focus = document.hasFocus ? document.hasFocus() : undefined;
				const cookieNames = parseCookies();
				snapshot.cookie_names = cookieNames;
				snapshot.cookie_contains_redirect_hint = cookieNames.includes(cookieHintName);
			}

			return snapshot;
		}

		function renderSummary(snapshot) {
			if (!diagSummary) {
				return;
			}
			const params = snapshot.query_params ?? {};
			const cookieNames = Array.isArray(snapshot.cookie_names) ? snapshot.cookie_names : [];
			const rows = [
				{ label: 'Snapshot generated', value: snapshot.generated_at || '—' },
				{ label: 'Current location', value: snapshot.location_href || '—' },
				{ label: 'Redirect query param', value: params.redirect_to || '—' },
				{ label: 'isMobile flag', value: params.isMobile || '—' },
				{ label: 'Redirect hint cookie', value: snapshot.cookie_contains_redirect_hint ? 'present' : 'missing' },
				{ label: 'Cookies detected', value: cookieNames.length ? cookieNames.join(', ') : 'None' },
				{ label: 'Document referrer', value: snapshot.document_referrer || 'None' },
				{ label: 'User agent', value: snapshot.navigator_user_agent || '—' },
			];
			diagSummary.innerHTML = rows
				.map((row) => '<div class="diag-item"><div class="diag-label">' + row.label + '</div><div class="diag-value">' + (row.value || '—') + '</div></div>')
				.join('');
		}

		function refreshSnapshot() {
			const snapshot = collectDiagnostics();
			if (diagJson) {
				diagJson.textContent = JSON.stringify(snapshot, null, 2);
			}
			renderSummary(snapshot);
		}

		function copyDiagnostics() {
			if (!diagJson) {
				return;
			}
			const text = diagJson.textContent || '';
			if (!text) {
				return;
			}
			const fallbackCopy = () => {
				const textarea = document.createElement('textarea');
				textarea.value = text;
				textarea.setAttribute('readonly', '');
				textarea.style.position = 'absolute';
				textarea.style.left = '-9999px';
				document.body.appendChild(textarea);
				textarea.select();
				try { document.execCommand('copy'); } catch (_) {}
				document.body.removeChild(textarea);
			};
			if (navigator?.clipboard?.writeText) {
				navigator.clipboard.writeText(text).catch(fallbackCopy);
				return;
			}
			fallbackCopy();
		}

		refreshBtn?.addEventListener('click', refreshSnapshot);
		copyBtn?.addEventListener('click', copyDiagnostics);
		refreshSnapshot();
	})();
	</script>
	</body>
</html>`, statusCopy, ctaMarkup, statusVariant, statusLabel, issuer, redirect, htmlEscape(pluginID), ready)
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

	// Check if this is a mobile/desktop client login
	isMobile := r.URL.Query().Get("isMobile") == "true"
	redirectTo := sanitizeRedirectTarget(r.URL.Query().Get("redirect_to"))
	referer := strings.TrimSpace(r.Referer())
	if resolved := fallbackRedirectFromCookie(w, r, redirectTo); resolved != redirectTo {
		p.API.LogDebug("redirect_to cookie fallback applied", "original", redirectTo, "resolved", resolved)
		redirectTo = resolved
	}
	if resolved := fallbackRedirectFromReferer(r, redirectTo); resolved != redirectTo {
		p.API.LogDebug("redirect_to referer fallback applied", "original", redirectTo, "resolved", resolved, "referer", referer)
		redirectTo = resolved
	}

	// Log the detection for debugging
	p.API.LogInfo("handleLogin called", "isMobile", isMobile, "query_params", r.URL.Query().Encode(), "user_agent", r.Header.Get("User-Agent"), "referer", referer)

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
		IsMobile:     isMobile,
		RedirectTo:   redirectTo,
	}
	if err := p.saveAuthSession(state, session); err != nil {
		p.API.LogError("failed to persist auth session", "error", err.Error())
		p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to start login", "We couldn't store the temporary login session. Please try again.")
		return
	}

	p.API.LogDebug("redirecting to OIDC provider", "state", state, "is_mobile", isMobile)

	// For mobile/desktop clients, render an HTML page that opens the OAuth URL in an external browser
	// This prevents the OAuth flow from happening in an embedded webview
	if isMobile {
		p.renderExternalBrowserRedirect(w, authorizeURL)
		return
	}

	// For web clients, use a standard HTTP redirect
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

	// For mobile/desktop clients, redirect to the completion endpoint with tokens
	if session.IsMobile {
		createdSession, err := p.createUserSession(user)
		if err != nil {
			p.API.LogError("failed to create session for mobile", "state", state, "error", err.Error())
			p.writeFriendlyError(w, http.StatusInternalServerError, "Unable to finish signing you in", "We couldn't establish a Mattermost session. Please try again.")
			return
		}

		if err := p.persistSessionTokens(createdSession, tokens, cfg); err != nil {
			p.API.LogWarn("failed to persist session tokens", "state", state, "error", err.Error())
		}

		p.API.LogDebug("authentication successful for mobile", "sub", profile.Subject, "user_id", user.Id)
		p.redirectToMobileComplete(w, r, createdSession, session.RedirectTo)
		return
	}

	createdSession, err := p.completeLogin(w, r, user, cfg, session.RedirectTo)
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

func (p *Plugin) completeLogin(w http.ResponseWriter, r *http.Request, user *model.User, cfg *Configuration, redirectTo string) (*model.Session, error) {
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

	redirectTo = sanitizeRedirectTarget(redirectTo)
	redirectTarget := postLoginRedirect(cfg, p.API.GetConfig())
	finalTarget := resolveRedirectURL(redirectTarget, redirectTo)
	http.Redirect(w, r, finalTarget, http.StatusFound)
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

func (p *Plugin) finalizeDesktopLogin(w http.ResponseWriter, r *http.Request, token, userID, csrfToken, expiresParam, redirectTo string) error {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(userID) == "" {
		http.Error(w, "Missing authentication parameters", http.StatusBadRequest)
		return fmt.Errorf("missing authentication parameters")
	}

	var expiresAt int64
	if strings.TrimSpace(expiresParam) != "" {
		parsed, err := strconv.ParseInt(expiresParam, 10, 64)
		if err != nil {
			p.API.LogWarn("invalid expiry parameter in desktop completion", "value", expiresParam)
		} else {
			expiresAt = parsed
		}
	}

	session := &model.Session{
		UserId:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
	}

	if strings.TrimSpace(csrfToken) != "" {
		session.AddProp("csrf", csrfToken)
	}

	cfg := p.getConfiguration()
	if err := issueSessionCookies(w, r, session, csrfToken, cfg); err != nil {
		http.Error(w, "Unable to finalize desktop login", http.StatusInternalServerError)
		return fmt.Errorf("issue session cookies: %w", err)
	}

	redirectTo = sanitizeRedirectTarget(redirectTo)
	redirectTarget := postLoginRedirect(cfg, p.API.GetConfig())
	finalTarget := resolveRedirectURL(redirectTarget, redirectTo)
	http.Redirect(w, r, finalTarget, http.StatusFound)
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

func sanitizeRedirectTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(raw, "//") {
		return ""
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		raw = parsed.Path
		if parsed.RawQuery != "" {
			raw = fmt.Sprintf("%s?%s", raw, parsed.RawQuery)
		}
		if parsed.Fragment != "" {
			raw = fmt.Sprintf("%s#%s", raw, parsed.Fragment)
		}
	}

	if !strings.HasPrefix(raw, "/") {
		return ""
	}

	if len(raw) > 2048 {
		raw = raw[:2048]
	}

	pluginRoot := fmt.Sprintf("/plugins/%s", pluginID)
	blocked := []string{
		pluginRoot + "/login",
		pluginRoot + "/complete",
	}
	for _, prefix := range blocked {
		if strings.HasPrefix(raw, prefix) {
			return "/"
		}
	}

	return raw
}

func fallbackRedirectFromCookie(w http.ResponseWriter, r *http.Request, current string) string {
	current = strings.TrimSpace(current)
	if current != "" && current != "/" {
		return current
	}
	hint := consumeRedirectHintCookie(w, r)
	if hint == "" {
		return current
	}
	return hint
}

func fallbackRedirectFromReferer(r *http.Request, current string) string {
	current = strings.TrimSpace(current)
	if current != "" && current != "/" {
		return current
	}
	if r == nil {
		return current
	}
	referer := strings.TrimSpace(r.Referer())
	if referer == "" {
		return current
	}
	parsed, err := url.Parse(referer)
	if err != nil {
		return current
	}
	path := strings.TrimSpace(parsed.Path)
	if path == "" {
		return current
	}
	if parsed.RawQuery != "" {
		path = fmt.Sprintf("%s?%s", path, parsed.RawQuery)
	}
	if parsed.Fragment != "" {
		path = fmt.Sprintf("%s#%s", path, parsed.Fragment)
	}
	sanitized := sanitizeRedirectTarget(path)
	if sanitized == "" || sanitized == "/" {
		return current
	}
	pluginRoot := fmt.Sprintf("/plugins/%s", pluginID)
	if strings.HasPrefix(sanitized, pluginRoot) {
		return current
	}
	return sanitized
}

func consumeRedirectHintCookie(w http.ResponseWriter, r *http.Request) string {
	if r == nil {
		return ""
	}
	cookie, err := r.Cookie(redirectHintCookie)
	if err != nil {
		return ""
	}
	expireRedirectHintCookie(w)
	value := strings.TrimSpace(cookie.Value)
	if value == "" {
		return ""
	}
	return sanitizeRedirectTarget(value)
}

func expireRedirectHintCookie(w http.ResponseWriter) {
	if w == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     redirectHintCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func resolveRedirectURL(baseURL, redirectPath string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "/"
	}
	if strings.TrimSpace(redirectPath) == "" {
		return baseURL
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	target, err := url.Parse(redirectPath)
	if err != nil {
		return baseURL
	}
	if target.Scheme != "" || target.Host != "" {
		return baseURL
	}

	parsedBase.Path = target.Path
	parsedBase.RawQuery = target.RawQuery
	parsedBase.Fragment = target.Fragment
	return parsedBase.String()
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

func hashForDiagnostics(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

// createUserSession creates a Mattermost session for the given user without setting cookies.
func (p *Plugin) createUserSession(user *model.User) (*model.Session, error) {
	session := &model.Session{UserId: user.Id, Roles: strings.TrimSpace(user.Roles)}
	if session.Roles == "" {
		session.Roles = model.SystemUserRoleId
	}
	session.PreSave()
	session.GenerateCSRF()
	session.Id = ""

	created, appErr := p.API.CreateSession(session)
	if appErr != nil {
		return nil, fmt.Errorf("create session: %w", appErr)
	}

	return created, nil
}

// redirectToMobileComplete redirects to the /complete endpoint with session tokens for mobile/desktop clients.
func (p *Plugin) redirectToMobileComplete(w http.ResponseWriter, r *http.Request, session *model.Session, redirectTo string) {
	cfg := p.getConfiguration()
	redirectPath := sanitizeRedirectTarget(redirectTo)

	// Build the complete URL with tokens as query parameters
	completeURL, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		p.API.LogError("failed to parse redirect URL", "error", err.Error())
		http.Error(w, "Configuration error", http.StatusInternalServerError)
		return
	}

	// Replace the path to point to our complete endpoint
	completeURL.Path = fmt.Sprintf("/plugins/%s/complete", pluginID)

	q := completeURL.Query()
	q.Set("MMAUTHTOKEN", session.Token)
	q.Set("MMUSERID", session.UserId)
	if csrf := strings.TrimSpace(session.GetCSRF()); csrf != "" {
		q.Set("MMCSRF", csrf)
	}
	if session != nil && session.ExpiresAt > 0 {
		q.Set("MMEXPIRES", strconv.FormatInt(session.ExpiresAt, 10))
	}
	if redirectPath != "" {
		q.Set("MMREDIRECT", redirectPath)
	}
	completeURL.RawQuery = q.Encode()

	http.Redirect(w, r, completeURL.String(), http.StatusFound)
}

// handleMobileComplete serves the completion page that mobile/desktop clients can parse.
func (p *Plugin) handleMobileComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	authToken := r.URL.Query().Get("MMAUTHTOKEN")
	userID := r.URL.Query().Get("MMUSERID")
	csrfToken := r.URL.Query().Get("MMCSRF")
	expiresParam := r.URL.Query().Get("MMEXPIRES")
	redirectPath := sanitizeRedirectTarget(r.URL.Query().Get("MMREDIRECT"))
	isDesktopCallback := r.URL.Query().Get("desktop") == "1"

	if authToken == "" || userID == "" {
		http.Error(w, "Missing authentication parameters", http.StatusBadRequest)
		return
	}

	if isDesktopCallback {
		if err := p.finalizeDesktopLogin(w, r, authToken, userID, csrfToken, expiresParam, redirectPath); err != nil {
			p.API.LogError("desktop login handoff failed", "error", err.Error())
		}
		return
	}

	cfg := p.getConfiguration()
	mmCfg := p.API.GetConfig()

	// Get the site URL for the mattermost:// redirect
	siteURL := "/"
	if mmCfg != nil && mmCfg.ServiceSettings.SiteURL != nil {
		if site := strings.TrimSpace(*mmCfg.ServiceSettings.SiteURL); site != "" {
			siteURL = site
		}
	} else if cfg != nil && strings.TrimSpace(cfg.RedirectURL) != "" {
		if parsed, err := url.Parse(cfg.RedirectURL); err == nil {
			parsed.Path = "/"
			parsed.RawQuery = ""
			parsed.Fragment = ""
			siteURL = parsed.String()
		}
	}

	// Build the mattermost:// URL for the desktop app
	params := url.Values{}
	params.Set("MMAUTHTOKEN", authToken)
	params.Set("MMUSERID", userID)
	if csrfToken != "" {
		params.Set("MMCSRF", csrfToken)
	}
	if expiresParam != "" {
		params.Set("MMEXPIRES", expiresParam)
	}
	if redirectPath != "" {
		params.Set("MMREDIRECT", redirectPath)
	}
	params.Set("desktop", "1")
	mattermostURL := fmt.Sprintf("mattermost://%s/plugins/%s/complete?%s",
		extractHost(siteURL), pluginID, params.Encode())

	expiresDisplay := "session-wide"
	if expiresParam != "" {
		if parsedExpires, err := strconv.ParseInt(expiresParam, 10, 64); err == nil && parsedExpires > 0 {
			expiresDisplay = time.UnixMilli(parsedExpires).UTC().Format(time.RFC3339)
		}
	}

	callbackBase := strings.TrimRight(siteURL, "/")
	if callbackBase == "" {
		callbackBase = siteURL
	}
	callbackHint := fmt.Sprintf("%s/plugins/%s/complete?desktop=1", callbackBase, pluginID)

	diagID := fmt.Sprintf("desktop-%d", time.Now().UnixNano())
	diagSnapshot := map[string]string{
		"request_id":            diagID,
		"request_time_utc":      time.Now().UTC().Format(time.RFC3339Nano),
		"raw_query":             r.URL.RawQuery,
		"user_agent":            strings.TrimSpace(r.UserAgent()),
		"site_url":              siteURL,
		"plugin_id":             pluginID,
		"deeplink":              mattermostURL,
		"token_length":          strconv.Itoa(len(authToken)),
		"token_sha256":          hashForDiagnostics(authToken),
		"csrf_length":           strconv.Itoa(len(csrfToken)),
		"csrf_sha256":           hashForDiagnostics(csrfToken),
		"expires_param":         expiresParam,
		"expires_display":       expiresDisplay,
		"desktop_callback_hint": callbackHint,
		"redirect_path":         redirectPath,
	}
	if referer := strings.TrimSpace(r.Referer()); referer != "" {
		diagSnapshot["referer"] = referer
	}

	if csrfToken == "" {
		diagSnapshot["csrf_sha256"] = ""
		diagSnapshot["csrf_length"] = "0"
	}

	p.API.LogInfo("rendering desktop redirect", "request_id", diagID, "query", r.URL.RawQuery, "token_hash", diagSnapshot["token_sha256"], "csrf_hash", diagSnapshot["csrf_sha256"], "expires", expiresParam, "redirect_path", redirectPath)

	diagJSON, _ := json.Marshal(diagSnapshot)
	diagJSEscaped := template.JSEscapeString(string(diagJSON))
	redirectDisplay := redirectPath
	if redirectDisplay == "" {
		redirectDisplay = "/"
	}

	// Render an HTML page that will trigger the desktop app redirect
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8" />
	<title>Redirecting to Mattermost App</title>
	<style>
		body { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; margin: 0; padding: 2rem; background: #0f172a; color: #f1f5f9; }
		.card { max-width: 640px; margin: 0 auto; background: rgba(15,23,42,0.85); border-radius: 16px; padding: 2rem; box-shadow: 0 15px 60px rgba(15,23,42,0.4); text-align: center; }
		h1 { margin-top: 0; font-size: 1.8rem; }
		p { line-height: 1.5; }
		.spinner { border: 4px solid rgba(56,189,248,0.2); border-top: 4px solid #38bdf8; border-radius: 50%%; width: 40px; height: 40px; animation: spin 1s linear infinite; margin: 2rem auto; }
		@keyframes spin { 0%% { transform: rotate(0deg); } 100%% { transform: rotate(360deg); } }
		.actions { margin-top: 1.5rem; }
		a.primary { display: inline-block; padding: 0.85rem 1.6rem; border-radius: 999px; font-weight: 600; background: #38bdf8; color: #0f172a; text-decoration: none; }
		a.primary:hover { opacity: 0.9; }
		.meta code { background: rgba(15,23,42,0.7); padding: 0.1rem 0.4rem; border-radius: 6px; font-size: 0.85rem; }
		details { margin-top: 1.25rem; text-align: left; background: rgba(15,23,42,0.6); border-radius: 12px; padding: 1rem; border: 1px solid rgba(148,163,184,0.2); }
		details summary { cursor: pointer; font-weight: 600; }
		.diag-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.75rem; margin-top: 1rem; }
		.label { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.08em; color: #94a3b8; }
		.value { font-family: monospace; word-break: break-all; font-size: 0.85rem; }
		.hint { font-size: 0.7rem; color: #cbd5f5; }
		pre { max-height: 220px; overflow: auto; background: rgba(2,6,23,0.85); padding: 0.75rem; border-radius: 8px; font-size: 0.8rem; }
		.diag-actions { margin: 0.5rem 0 1rem; display: flex; gap: 0.5rem; flex-wrap: wrap; }
		button.copy { background: transparent; border: 1px solid rgba(148,163,184,0.4); border-radius: 8px; color: #f1f5f9; padding: 0.4rem 0.8rem; cursor: pointer; }
		button.copy:hover { border-color: #38bdf8; color: #38bdf8; }
	</style>
	<script>
		const deepLinkTarget = %q;
		const diagSnapshot = JSON.parse('%s');
		let attemptCount = 0;

		function updateDiagnosticsView() {
			diagSnapshot.visibilityState = document.visibilityState;
			diagSnapshot.navigatorUserAgent = window.navigator.userAgent;
			const pre = document.getElementById('diagnostics-json');
			if (pre) {
				pre.textContent = JSON.stringify(diagSnapshot, null, 2);
			}
		}

		function triggerDeepLink() {
			attemptCount += 1;
			diagSnapshot.lastAttemptAt = new Date().toISOString();
			window.location.href = deepLinkTarget;
			const attemptEl = document.getElementById('attempt-count');
			if (attemptEl) {
				attemptEl.textContent = attemptCount.toString();
			}
			updateDiagnosticsView();
		}

		function copyDiagnostics() {
			const pre = document.getElementById('diagnostics-json');
			if (!pre) {
				return;
			}
			const text = pre.textContent || '';
			if (!text) {
				return;
			}
			function legacyCopy() {
				const temp = document.createElement('textarea');
				temp.value = text;
				temp.setAttribute('readonly', '');
				temp.style.position = 'absolute';
				temp.style.left = '-9999px';
				document.body.appendChild(temp);
				temp.select();
				try {
					document.execCommand('copy');
				} catch (err) {}
				document.body.removeChild(temp);
			}
			if (navigator.clipboard && navigator.clipboard.writeText) {
				navigator.clipboard.writeText(text).catch(function() {
					legacyCopy();
				});
			} else {
				legacyCopy();
			}
		}

		window.onload = function() {
			triggerDeepLink();
			setTimeout(function() {
				document.getElementById('manual-link').style.display = 'block';
				document.getElementById('spinner').style.display = 'none';
			}, 3000);
			const copyButton = document.getElementById('copy-diagnostics');
			if (copyButton) {
				copyButton.addEventListener('click', function(event) {
					event.preventDefault();
					copyDiagnostics();
				});
			}
			const retryButton = document.getElementById('retry-deeplink');
			if (retryButton) {
				retryButton.addEventListener('click', function(event) {
					event.preventDefault();
					triggerDeepLink();
				});
			}
			setInterval(updateDiagnosticsView, 2000);
			updateDiagnosticsView();
		};

		document.addEventListener('visibilitychange', updateDiagnosticsView);
	</script>
</head>
<body>
	<main class="card">
		<h1>Redirecting to Mattermost</h1>
		<div id="spinner" class="spinner"></div>
		<p>Opening the Mattermost desktop app...</p>
		<p class="meta">Session handoff window expires: <strong>%s</strong></p>
		<p class="meta">Return target after login: <code>%s</code></p>
		<p class="meta">Handoff ID: <code>%s</code></p>
		<p class="meta">Auto-attempts triggered: <strong><span id="attempt-count">0</span></strong></p>
		<details class="diagnostics" open>
			<summary>Diagnostics payload</summary>
			<div class="diag-grid">
				<div>
					<div class="label">Token SHA256</div>
					<div class="value">%s</div>
					<div class="hint">Length: %s</div>
				</div>
				<div>
					<div class="label">CSRF SHA256</div>
					<div class="value">%s</div>
					<div class="hint">Length: %s</div>
				</div>
				<div>
					<div class="label">Raw expires param</div>
					<div class="value">%s</div>
				</div>
				<div>
					<div class="label">Redirect path</div>
					<div class="value">%s</div>
				</div>
			</div>
			<div class="diag-actions">
				<button id="copy-diagnostics" class="copy">Copy JSON snapshot</button>
				<button id="retry-deeplink" class="copy">Retry deep link</button>
			</div>
			<pre id="diagnostics-json">Collecting...</pre>
				</details>
				<div id="manual-link" class="actions" style="display: none;">
			<p>If the app didn't open automatically:</p>
			<a class="primary" href="%s">Click here to open Mattermost</a>
		</div>
	</main>
</body>
		</html>`, mattermostURL, diagJSEscaped, expiresDisplay, redirectDisplay, diagSnapshot["request_id"], diagSnapshot["token_sha256"], diagSnapshot["token_length"], diagSnapshot["csrf_sha256"], diagSnapshot["csrf_length"], diagSnapshot["expires_param"], diagSnapshot["redirect_path"], mattermostURL)
}

// extractHost extracts the host from a URL string for use in mattermost:// protocol.
func extractHost(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "localhost"
	}
	if parsed.Host != "" {
		return parsed.Host
	}
	return "localhost"
}

// renderExternalBrowserRedirect renders an HTML page that opens the OAuth URL in an external browser window.
// This is used for mobile/desktop clients to ensure OAuth happens in the system browser, not an embedded webview.

func (p *Plugin) renderExternalBrowserRedirect(w http.ResponseWriter, oauthURL string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Escape for safe attribute inclusion; the DOM will normalize hrefs before JS reads them
	escapedURL := htmlEscape(oauthURL)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8" />
	<title>Opening External Browser</title>
	<style>
		body { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; margin: 0; padding: 2rem; background: #0f172a; color: #f1f5f9; }
		.card { max-width: 640px; margin: 0 auto; background: rgba(15,23,42,0.85); border-radius: 16px; padding: 2rem; box-shadow: 0 15px 60px rgba(15,23,42,0.4); text-align: center; }
		h1 { margin-top: 0; font-size: 1.8rem; }
		p { line-height: 1.5; }
		.spinner { border: 4px solid rgba(56,189,248,0.2); border-top: 4px solid #38bdf8; border-radius: 50%%; width: 40px; height: 40px; animation: spin 1s linear infinite; margin: 2rem auto; }
		@keyframes spin { 0%% { transform: rotate(0deg); } 100%% { transform: rotate(360deg); } }
		a.primary { display: inline-block; padding: 0.85rem 1.6rem; border-radius: 999px; font-weight: 600; background: #38bdf8; color: #0f172a; text-decoration: none; margin-top: 1.5rem; }
		a.primary:hover { opacity: 0.9; }
	</style>
	<script>
		function launchExternalBrowser() {
			var link = document.getElementById('oauth-link');
			if (!link || !link.href) {
				return;
			}
			window.open(link.href, '_blank', 'noopener');
		}
		// Automatically try to open the OAuth URL in an external browser
		// The desktop app should intercept this and open it in the system browser
		window.onload = function() {
			launchExternalBrowser();
			
			// Show fallback link after a short delay
			setTimeout(function() {
				document.getElementById('fallback').style.display = 'block';
				document.getElementById('spinner').style.display = 'none';
			}, 2000);
		};
	</script>
</head>
<body>
	<a id="oauth-link" href="%s" rel="noreferrer noopener" style="display:none;"></a>
	<main class="card">
		<h1>Opening Browser for Authentication</h1>
		<div id="spinner" class="spinner"></div>
		<p>Opening your system browser to complete authentication...</p>
		<p style="font-size: 0.9rem; color: #cbd5e1;">Please complete the login in your browser, then return to this window.</p>
		<div id="fallback" style="display: none;">
			<p style="color: #f87171;">If the browser didn't open automatically:</p>
			<a class="primary" href="%s" target="_blank" rel="noreferrer noopener">Click here to open browser</a>
		</div>
	</main>
</body>
</html>`, escapedURL, escapedURL)
}
