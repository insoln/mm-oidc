package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsValidMobileRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "valid custom scheme",
			url:      "mattermost://callback",
			expected: true,
		},
		{
			name:     "valid custom scheme with host and path",
			url:      "mattermostdesktop://auth/complete?foo=bar",
			expected: true,
		},
		{
			name:     "invalid http scheme",
			url:      "http://example.com",
			expected: false,
		},
		{
			name:     "invalid https scheme",
			url:      "https://example.com",
			expected: false,
		},
		{
			name:     "empty url",
			url:      "",
			expected: false,
		},
		{
			name:     "whitespace url",
			url:      "   ",
			expected: false,
		},
		{
			name:     "invalid url format",
			url:      "not a url",
			expected: false,
		},
		{
			name:     "no scheme",
			url:      "example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidMobileRedirectURL(tt.url)
			if result != tt.expected {
				t.Errorf("isValidMobileRedirectURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestBuildMobileCallbackURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		expected string
	}{
		{
			name:     "callback path",
			base:     "http://localhost:8065/plugins/com.mm.oidc/callback",
			expected: "http://localhost:8065/plugins/com.mm.oidc/callback/mobile",
		},
		{
			name:     "base without callback",
			base:     "http://localhost:8065/plugins/com.mm.oidc",
			expected: "http://localhost:8065/plugins/com.mm.oidc/callback/mobile",
		},
		{
			name:     "with query params",
			base:     "http://localhost:8065/plugins/com.mm.oidc/callback?foo=bar",
			expected: "http://localhost:8065/plugins/com.mm.oidc/callback/mobile?foo=bar",
		},
		{
			name:     "invalid url returns modified",
			base:     "not a url",
			expected: "not%20a%20url/callback/mobile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildMobileCallbackURL(tt.base)
			if result != tt.expected {
				t.Errorf("buildMobileCallbackURL(%q) = %q, want %q", tt.base, result, tt.expected)
			}
		})
	}
}

func TestBuildMobileRedirectURL(t *testing.T) {
	tests := []struct {
		name         string
		appURL       string
		sessionToken string
		csrfToken    string
		wantContains []string
	}{
		{
			name:         "basic custom scheme",
			appURL:       "mattermost://callback",
			sessionToken: "token123",
			csrfToken:    "csrf456",
			wantContains: []string{
				"mattermost://callback",
				"MMAUTHTOKEN=token123",
				"MMCSRF=csrf456",
			},
		},
		{
			name:         "with existing query params",
			appURL:       "mattermost://callback?existing=param",
			sessionToken: "token123",
			csrfToken:    "csrf456",
			wantContains: []string{
				"mattermost://callback",
				"existing=param",
				"MMAUTHTOKEN=token123",
				"MMCSRF=csrf456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildMobileRedirectURL(tt.appURL, tt.sessionToken, tt.csrfToken)
			for _, want := range tt.wantContains {
				if !contains(result, want) {
					t.Errorf("buildMobileRedirectURL() result %q does not contain %q", result, want)
				}
			}
		})
	}
}

func TestHandleMobileLoginMissingRedirectTo(t *testing.T) {
	plugin := &Plugin{
		configuration: &Configuration{
			IssuerURL:    "https://keycloak.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://localhost:8065/plugins/com.mm.oidc/callback",
			Scopes:       []string{"openid", "profile", "email"},
		},
		metadata: &OIDCMetadata{
			Issuer:                "https://keycloak.example.com",
			AuthorizationEndpoint: "https://keycloak.example.com/auth",
			TokenEndpoint:         "https://keycloak.example.com/token",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/login/mobile", nil)
	w := httptest.NewRecorder()

	plugin.handleMobileLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleMobileLogin() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	body := w.Body.String()
	if !contains(body, "redirect_to") {
		t.Errorf("handleMobileLogin() body should mention redirect_to parameter")
	}
}

func TestHandleMobileLoginInvalidRedirectTo(t *testing.T) {
	plugin := &Plugin{
		configuration: &Configuration{
			IssuerURL:    "https://keycloak.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://localhost:8065/plugins/com.mm.oidc/callback",
			Scopes:       []string{"openid", "profile", "email"},
		},
		metadata: &OIDCMetadata{
			Issuer:                "https://keycloak.example.com",
			AuthorizationEndpoint: "https://keycloak.example.com/auth",
			TokenEndpoint:         "https://keycloak.example.com/token",
		},
	}

	// Test with http:// scheme which should be invalid for mobile
	req := httptest.NewRequest(http.MethodGet, "/login/mobile?redirect_to=http://example.com", nil)
	w := httptest.NewRecorder()

	plugin.handleMobileLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleMobileLogin() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	body := w.Body.String()
	if !contains(body, "Invalid redirect URL") {
		t.Errorf("handleMobileLogin() body should mention invalid redirect URL")
	}
}

func TestRenderMobileAuthComplete(t *testing.T) {
	w := httptest.NewRecorder()
	redirectURL := "mattermost://callback?token=abc123"

	renderMobileAuthComplete(w, redirectURL)

	if w.Code != http.StatusOK {
		t.Errorf("renderMobileAuthComplete() status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	expectedParts := []string{
		"Authentication Complete",
		"mattermost://callback",
		"token=abc123",
		"meta http-equiv=\"refresh\"",
		"Redirecting back to your application",
	}

	for _, part := range expectedParts {
		if !contains(body, part) {
			t.Errorf("renderMobileAuthComplete() body missing %q", part)
		}
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("renderMobileAuthComplete() content-type = %q, want %q", contentType, "text/html; charset=utf-8")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
