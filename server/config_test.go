package main

import "testing"

func TestConfigurationValidate(t *testing.T) {
	tests := map[string]struct {
		config  Configuration
		wantErr bool
	}{
		"valid config": {
			config: Configuration{
				IssuerURL:    "https://sso.example.com",
				ClientID:     "mm-oidc",
				ClientSecret: "super-secret",
				RedirectURL:  "https://chat.example.com/plugins/com.mm.oidc/oauth/callback",
				Scopes:       []string{"openid", "profile"},
			},
		},
		"missing issuer": {
			config: Configuration{
				ClientID:     "mm-oidc",
				ClientSecret: "super-secret",
				RedirectURL:  "https://chat.example.com/callback",
				Scopes:       []string{"openid"},
			},
			wantErr: true,
		},
		"insecure issuer rejected": {
			config: Configuration{
				IssuerURL:    "http://idp.local",
				ClientID:     "mm-oidc",
				ClientSecret: "super-secret",
				RedirectURL:  "https://chat.example.com/callback",
				Scopes:       []string{"openid"},
			},
			wantErr: true,
		},
		"allow insecure issuer": {
			config: Configuration{
				IssuerURL:           "http://idp.local",
				ClientID:            "mm-oidc",
				ClientSecret:        "super-secret",
				RedirectURL:         "https://chat.example.com/callback",
				Scopes:              []string{"openid"},
				AllowInsecureIssuer: true,
			},
			wantErr: false,
		},
		"missing scopes": {
			config: Configuration{
				IssuerURL:    "https://sso.example.com",
				ClientID:     "mm-oidc",
				ClientSecret: "super-secret",
				RedirectURL:  "https://chat.example.com/callback",
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
