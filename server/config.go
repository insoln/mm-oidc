package main

import (
	"fmt"
	"net/url"
	"strings"
)

// Configuration mirrors the System Console fields exposed to administrators.
// It deliberately focuses on the minimal values required to drive the OIDC code flow.
type Configuration struct {
	IssuerURL           string   `json:"issuer_url"`
	ClientID            string   `json:"client_id"`
	ClientSecret        string   `json:"client_secret"`
	RedirectURL         string   `json:"redirect_url"`
	Scopes              []string `json:"scopes"`
	AllowInsecureIssuer bool     `json:"allow_insecure_issuer"`
	EnableAutoRedirect  bool     `json:"enable_auto_redirect"`
	ShowLoginButton     bool     `json:"show_login_button"`
}

// Clone returns a deep copy so callers can safely mutate without holding locks.
func (c *Configuration) Clone() *Configuration {
	if c == nil {
		return &Configuration{}
	}

	clone := *c
	if len(c.Scopes) > 0 {
		clone.Scopes = append([]string(nil), c.Scopes...)
	}
	return &clone
}

// SetDefaults ensures optional values are initialized.
func (c *Configuration) SetDefaults() {
	if len(c.Scopes) == 0 {
		c.Scopes = []string{"openid", "profile", "email"}
	}
	// ShowLoginButton defaults to true if not explicitly set
	// This is handled by plugin.json default, but we ensure it here too
}

// Validate performs basic semantic checks and protects against misconfiguration before activation.
func (c *Configuration) Validate() error {
	if c == nil {
		return fmt.Errorf("configuration is nil")
	}

	if strings.TrimSpace(c.IssuerURL) == "" {
		return fmt.Errorf("issuer_url is required")
	}

	issuer, err := url.Parse(c.IssuerURL)
	if err != nil {
		return fmt.Errorf("issuer_url is invalid: %w", err)
	}

	if issuer.Scheme != "https" && !c.AllowInsecureIssuer {
		return fmt.Errorf("issuer_url must be https (set allow_insecure_issuer for dev overrides)")
	}

	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("client_id is required")
	}

	if strings.TrimSpace(c.ClientSecret) == "" {
		return fmt.Errorf("client_secret is required")
	}

	if strings.TrimSpace(c.RedirectURL) == "" {
		return fmt.Errorf("redirect_url is required")
	}

	if _, err := url.ParseRequestURI(c.RedirectURL); err != nil {
		return fmt.Errorf("redirect_url is invalid: %w", err)
	}

	if len(c.Scopes) == 0 {
		return fmt.Errorf("scopes must contain at least one entry")
	}

	return nil
}
