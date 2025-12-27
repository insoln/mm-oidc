package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	discoveryTimeout      = 5 * time.Second
	tokenExchangeTimeout  = 10 * time.Second
	authSessionTTLSeconds = 300
	authSessionMaxAge     = time.Duration(authSessionTTLSeconds) * time.Second
	stateBytes            = 32
	nonceBytes            = 32
	pkceVerifierBytes     = 64
)

// OIDCMetadata captures a subset of the provider discovery document.
type OIDCMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSEndpoint          string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// authSession tracks temporary state between /login and /callback.
type authSession struct {
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	CreatedAt    int64  `json:"created_at"`
	DesktopToken string `json:"desktop_token,omitempty"`
	RedirectTo   string `json:"redirect_to,omitempty"`
	IsMobile     bool   `json:"is_mobile,omitempty"`
}

func (s *authSession) isExpired(now time.Time) bool {
	if s == nil {
		return true
	}

	if s.CreatedAt == 0 {
		return true
	}

	created := time.Unix(s.CreatedAt, 0)
	return now.Sub(created) > authSessionMaxAge
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func buildDiscoveryURL(issuer string) (string, error) {
	if strings.TrimSpace(issuer) == "" {
		return "", fmt.Errorf("issuer URL is empty")
	}

	parsed, err := url.Parse(issuer)
	if err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}

	cleanPath := strings.Trim(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	fullPath := path.Join(cleanPath, ".well-known", "openid-configuration")
	parsed.Path = "/" + strings.TrimPrefix(fullPath, "/")
	return parsed.String(), nil
}

func buildAuthorizeURL(base string, cfg *Configuration, state, nonce, codeChallenge string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("authorization endpoint missing")
	}

	endpoint, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid authorization endpoint: %w", err)
	}

	q := endpoint.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	q.Set("scope", strings.Join(cfg.Scopes, " "))
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	endpoint.RawQuery = q.Encode()
	return endpoint.String(), nil
}

func generateRandomString(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (p *Plugin) fetchMetadata(ctx context.Context, issuer string) (*OIDCMetadata, error) {
	discoveryURL, err := buildDiscoveryURL(issuer)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create discovery request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discover provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery request failed: %s", resp.Status)
	}

	var metadata OIDCMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode discovery response: %w", err)
	}

	if metadata.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("discovery document missing authorization_endpoint")
	}

	return &metadata, nil
}

func (p *Plugin) saveAuthSession(state string, session *authSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal auth session: %w", err)
	}

	if appErr := p.API.KVSetWithExpiry(authSessionKey(state), payload, authSessionTTLSeconds); appErr != nil {
		return fmt.Errorf("persist auth session: %w", appErr)
	}

	return nil
}

func (p *Plugin) consumeAuthSession(state string) (*authSession, error) {
	data, appErr := p.API.KVGet(authSessionKey(state))
	if appErr != nil {
		return nil, fmt.Errorf("load auth session: %w", appErr)
	}
	if data == nil {
		return nil, nil
	}

	if err := p.API.KVDelete(authSessionKey(state)); err != nil {
		p.API.LogWarn("failed to delete auth session", "state", state, "error", err.Error())
	}

	var session authSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("decode auth session: %w", err)
	}

	if session.isExpired(time.Now()) {
		return nil, nil
	}

	return &session, nil
}

func authSessionKey(state string) string {
	return "authsession:" + state
}

func (p *Plugin) setMetadata(metadata *OIDCMetadata) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()
	if metadata == nil {
		p.metadata = nil
		return
	}
	copy := *metadata
	p.metadata = &copy
}

func (p *Plugin) refreshMetadata(cfg *Configuration) error {
	if p.httpClient == nil {
		return fmt.Errorf("http client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()

	metadata, err := p.fetchMetadata(ctx, cfg.IssuerURL)
	if err != nil {
		return err
	}

	p.setMetadata(metadata)
	return nil
}

func (p *Plugin) initOIDCProvider(cfg *Configuration) error {
	if p.httpClient == nil {
		return fmt.Errorf("http client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()
	ctx = oidc.ClientContext(ctx, p.httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return fmt.Errorf("bootstrap oidc provider: %w", err)
	}

	p.setProvider(provider)
	return nil
}

func (p *Plugin) exchangeCode(ctx context.Context, cfg *Configuration, metadata *OIDCMetadata, code, codeVerifier string) (*tokenResponse, error) {
	if metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("token endpoint missing in discovery metadata")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("token endpoint responded %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if tokens.IDToken == "" {
		return nil, fmt.Errorf("token response missing id_token")
	}

	return &tokens, nil
}

func (p *Plugin) verifyIDToken(ctx context.Context, provider *oidc.Provider, cfg *Configuration, rawIDToken, expectedNonce string) (*idTokenClaims, error) {
	if rawIDToken == "" {
		return nil, fmt.Errorf("id_token missing")
	}

	ctx = oidc.ClientContext(ctx, p.httpClient)
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	var claims idTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode id_token claims: %w", err)
	}

	if expectedNonce != "" && claims.Nonce != expectedNonce {
		return nil, fmt.Errorf("nonce mismatch")
	}

	return &claims, nil
}
