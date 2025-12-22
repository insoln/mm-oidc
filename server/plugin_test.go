package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestPostLoginRedirectPrefersSiteURL(t *testing.T) {
	cfg := &Configuration{RedirectURL: "https://mm.example.com/plugins/mm-oidc/callback"}
	mmCfg := &model.Config{}
	mmCfg.ServiceSettings.SiteURL = model.NewPointer("https://site.example.com")

	got := postLoginRedirect(cfg, mmCfg)
	if got != "https://site.example.com" {
		t.Fatalf("expected site URL redirect, got %s", got)
	}
}

func TestPostLoginRedirectFallbacks(t *testing.T) {
	cfg := &Configuration{RedirectURL: "https://mm.example.com/plugins/mm-oidc/callback"}

	t.Run("uses redirect base", func(t *testing.T) {
		got := postLoginRedirect(cfg, nil)
		if got != "https://mm.example.com/" {
			t.Fatalf("expected redirect base, got %s", got)
		}
	})

	t.Run("defaults to root", func(t *testing.T) {
		got := postLoginRedirect(&Configuration{}, nil)
		if got != "/" {
			t.Fatalf("expected root redirect, got %s", got)
		}
	})
}

func TestRequestIsSecure(t *testing.T) {
	t.Run("tls request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
		req.TLS = &tls.ConnectionState{}
		if !requestIsSecure(req, nil) {
			t.Fatalf("expected secure request via TLS")
		}
	})

	t.Run("forwarded proto", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		if !requestIsSecure(req, nil) {
			t.Fatalf("expected secure request via forwarded proto")
		}
	})

	t.Run("config fallback", func(t *testing.T) {
		cfg := &Configuration{RedirectURL: "https://mm.example.com/plugins/mm-oidc/callback"}
		if !requestIsSecure(nil, cfg) {
			t.Fatalf("expected secure via config redirect")
		}
	})

	t.Run("insecure default", func(t *testing.T) {
		cfg := &Configuration{RedirectURL: "http://localhost:8065/plugins/mm-oidc/callback"}
		if requestIsSecure(nil, cfg) {
			t.Fatalf("expected insecure when no TLS hints")
		}
	})
}

func TestSessionCookieLifetime(t *testing.T) {
	session := &model.Session{ExpiresAt: model.GetMillis() + 60000}
	expires, maxAge := sessionCookieLifetime(session)
	if expires.IsZero() {
		t.Fatalf("expected expiration timestamp")
	}
	if maxAge <= 0 {
		t.Fatalf("expected positive max-age, got %d", maxAge)
	}

	expires, maxAge = sessionCookieLifetime(&model.Session{ExpiresAt: model.GetMillis() - 1000})
	if !expires.IsZero() {
		t.Fatalf("expected zero time for expired session")
	}
	if maxAge != 0 {
		t.Fatalf("expected zero max-age for expired session, got %d", maxAge)
	}
}

func TestIssueSessionCookies(t *testing.T) {
	session := &model.Session{
		Token:     "token",
		UserId:    "user",
		ExpiresAt: model.GetMillis() + int64(time.Hour/time.Millisecond),
	}
	req := httptest.NewRequest(http.MethodGet, "https://mm.example.com", nil)
	rec := httptest.NewRecorder()
	cfg := &Configuration{RedirectURL: "https://mm.example.com/plugins/mm-oidc/callback"}

	if err := issueSessionCookies(rec, req, session, "csrf-token", cfg); err != nil {
		t.Fatalf("issueSessionCookies failed: %v", err)
	}

	result := rec.Result()
	defer result.Body.Close()

	if cookie := cookieByName(result.Cookies(), model.SessionCookieToken); cookie == nil {
		t.Fatalf("expected MMAUTHTOKEN cookie")
	} else {
		if !cookie.HttpOnly {
			t.Fatalf("expected auth cookie to be HttpOnly")
		}
		if cookie.Secure != true {
			t.Fatalf("expected secure auth cookie")
		}
		if cookie.Value != "token" {
			t.Fatalf("unexpected token value: %s", cookie.Value)
		}
	}

	if cookie := cookieByName(result.Cookies(), model.SessionCookieUser); cookie == nil {
		t.Fatalf("expected MMUSERID cookie")
	} else if cookie.HttpOnly {
		t.Fatalf("expected user cookie to be readable by JS")
	}

	if cookie := cookieByName(result.Cookies(), model.SessionCookieCsrf); cookie == nil {
		t.Fatalf("expected MMCSRF cookie")
	} else if cookie.HttpOnly {
		t.Fatalf("csrf cookie should not be HttpOnly")
	}
}

func TestIssueSessionCookiesRequiresSession(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://mm.example.com", nil)
	if err := issueSessionCookies(rec, req, nil, "", &Configuration{}); err == nil {
		t.Fatalf("expected error for nil session")
	}
}

func TestPluginLandingURL(t *testing.T) {
	cfg := &Configuration{RedirectURL: "https://mm.example.com/plugins/com.mm.oidc/callback"}
	got := pluginLandingURL(cfg)
	if got != "https://mm.example.com/plugins/com.mm.oidc" {
		t.Fatalf("unexpected landing URL: %s", got)
	}

	got = pluginLandingURL(&Configuration{})
	if got != "/plugins/com.mm.oidc" {
		t.Fatalf("expected relative fallback, got %s", got)
	}
}

func TestBuildLogoutURL(t *testing.T) {
	logoutURL, err := buildLogoutURL("https://idp.example.com/logout", "id-token", "https://mm.example.com/plugins/com.mm.oidc")
	if err != nil {
		t.Fatalf("buildLogoutURL failed: %v", err)
	}

	parsed, err := url.Parse(logoutURL)
	if err != nil {
		t.Fatalf("parse logout url: %v", err)
	}

	q := parsed.Query()
	if q.Get("id_token_hint") != "id-token" {
		t.Fatalf("missing id_token_hint")
	}
	if q.Get("post_logout_redirect_uri") != "https://mm.example.com/plugins/com.mm.oidc" {
		t.Fatalf("missing redirect uri")
	}
	if q.Get("state") == "" {
		t.Fatalf("expected state param")
	}
}

func TestSessionTokenFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://mm.example.com", nil)
	req.AddCookie(&http.Cookie{Name: model.SessionCookieToken, Value: "cookie-token"})
	if got := sessionTokenFromRequest(req); got != "cookie-token" {
		t.Fatalf("expected cookie token, got %s", got)
	}

	req = httptest.NewRequest(http.MethodGet, "https://mm.example.com", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	if got := sessionTokenFromRequest(req); got != "header-token" {
		t.Fatalf("expected bearer token, got %s", got)
	}

	req = httptest.NewRequest(http.MethodGet, "https://mm.example.com", nil)
	req.Header.Set(model.HeaderToken, "legacy-token")
	if got := sessionTokenFromRequest(req); got != "legacy-token" {
		t.Fatalf("expected header token, got %s", got)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "super-secret"
	plaintext := []byte("payload")
	ciphertext, err := encryptForStorage(plaintext, secret)
	if err != nil {
		t.Fatalf("encryptForStorage failed: %v", err)
	}

	decrypted, err := decryptFromStorage(ciphertext, secret)
	if err != nil {
		t.Fatalf("decryptFromStorage failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("round trip mismatch: %s", decrypted)
	}

	if _, err := encryptForStorage([]byte(""), secret); err == nil {
		t.Fatalf("expected error for empty plaintext")
	}
	if _, err := encryptForStorage(plaintext, ""); err == nil {
		t.Fatalf("expected error for empty secret")
	}
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name     string
		urlStr   string
		expected string
	}{
		{
			name:     "full URL",
			urlStr:   "https://mattermost.example.com:8065/path",
			expected: "mattermost.example.com:8065",
		},
		{
			name:     "URL with default port",
			urlStr:   "https://mattermost.example.com",
			expected: "mattermost.example.com",
		},
		{
			name:     "localhost",
			urlStr:   "http://localhost:8065",
			expected: "localhost:8065",
		},
		{
			name:     "invalid URL",
			urlStr:   "not a valid url",
			expected: "localhost",
		},
		{
			name:     "empty URL",
			urlStr:   "",
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHost(tt.urlStr)
			if got != tt.expected {
				t.Errorf("extractHost(%q) = %q, want %q", tt.urlStr, got, tt.expected)
			}
		})
	}
}

func TestAuthSessionMobile(t *testing.T) {
	t.Run("stores mobile flag", func(t *testing.T) {
		session := &authSession{
			Nonce:        "test-nonce",
			CodeVerifier: "test-verifier",
			CreatedAt:    time.Now().Unix(),
			IsMobile:     true,
		}

		if !session.IsMobile {
			t.Fatal("expected IsMobile to be true")
		}
	})

	t.Run("defaults to web client", func(t *testing.T) {
		session := &authSession{
			Nonce:        "test-nonce",
			CodeVerifier: "test-verifier",
			CreatedAt:    time.Now().Unix(),
		}

		if session.IsMobile {
			t.Fatal("expected IsMobile to be false by default")
		}
	})
}
