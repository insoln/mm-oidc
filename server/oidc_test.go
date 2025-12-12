package main

import (
	"net/url"
	"testing"
	"time"
)

func TestBuildDiscoveryURL(t *testing.T) {
	tests := map[string]string{
		"root":       "https://example.com/.well-known/openid-configuration",
		"realm path": "https://idp.local/auth/realms/master/.well-known/openid-configuration",
	}

	inputs := map[string]string{
		"root":       "https://example.com",
		"realm path": "https://idp.local/auth/realms/master/",
	}

	for name, input := range inputs {
		want := tests[name]
		got, err := buildDiscoveryURL(input)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s: expected %s, got %s", name, want, got)
		}
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	cfg := &Configuration{
		ClientID:    "mm-oidc",
		RedirectURL: "https://chat.example.com/plugins/com.mm.oidc/callback",
		Scopes:      []string{"openid", "profile"},
	}

	authorize, err := buildAuthorizeURL("https://idp/auth", cfg, "state123", "nonce456", "challenge789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := url.Parse(authorize)
	if err != nil {
		t.Fatalf("failed to parse authorize URL: %v", err)
	}

	q := parsed.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != cfg.ClientID {
		t.Fatalf("missing core parameters")
	}
	if q.Get("redirect_uri") != cfg.RedirectURL {
		t.Fatalf("redirect URI mismatch")
	}
	if q.Get("scope") != "openid profile" {
		t.Fatalf("scope mismatch: %s", q.Get("scope"))
	}
	if q.Get("state") != "state123" || q.Get("nonce") != "nonce456" {
		t.Fatalf("state/nonce mismatch")
	}
	if q.Get("code_challenge") != "challenge789" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("code challenge missing")
	}
}

func TestPKCEChallenge(t *testing.T) {
	verifier := "test-verifier"
	challenge := pkceChallenge(verifier)
	if challenge == "" {
		t.Fatalf("expected non-empty challenge")
	}
	if challenge != pkceChallenge(verifier) {
		t.Fatalf("challenge should be deterministic")
	}
}

func TestBuildUserProfile(t *testing.T) {
	claims := &idTokenClaims{
		Subject:           "12345",
		Email:             "User@Example.com",
		EmailVerified:     true,
		PreferredUsername: "cool-user",
		GivenName:         "Test",
		FamilyName:        "User",
		Name:              "Test User",
		ResourceAccess: map[string]clientRoleMapping{
			"mm-oidc": {Roles: []string{"system_admin"}},
		},
	}

	profile := buildUserProfile(claims, "mm-oidc")
	if profile.Subject != "12345" || profile.Username != "cool-user" {
		t.Fatalf("unexpected profile normalization")
	}
	if profile.Email != "user@example.com" {
		t.Fatalf("email should be lowercased, got %s", profile.Email)
	}
	if !profile.SystemAdmin {
		t.Fatalf("expected system admin flag when client role granted")
	}

	claims.PreferredUsername = ""
	profile = buildUserProfile(claims, "mm-oidc")
	if profile.Username != "user@example.com" {
		t.Fatalf("expected email fallback for username")
	}

	claims.Email = ""
	profile = buildUserProfile(claims, "mm-oidc")
	if profile.Username != "12345" {
		t.Fatalf("expected subject fallback for username")
	}
	profile = buildUserProfile(claims, "other-client")
	if profile.SystemAdmin {
		t.Fatalf("client role flag should depend on client id")
	}
}

func TestAuthSessionExpiry(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	session := &authSession{CreatedAt: base.Unix()}
	if session.isExpired(base.Add(authSessionMaxAge - time.Second)) {
		t.Fatalf("session should be valid within max age")
	}
	if !session.isExpired(base.Add(authSessionMaxAge + time.Second)) {
		t.Fatalf("session should expire after max age")
	}
}
