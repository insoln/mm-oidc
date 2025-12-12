package main

import "strings"

// idTokenClaims captures standard OpenID Connect fields used for provisioning.
type idTokenClaims struct {
	Subject           string                       `json:"sub"`
	Email             string                       `json:"email"`
	EmailVerified     bool                         `json:"email_verified"`
	PreferredUsername string                       `json:"preferred_username"`
	GivenName         string                       `json:"given_name"`
	FamilyName        string                       `json:"family_name"`
	Name              string                       `json:"name"`
	Nonce             string                       `json:"nonce"`
	ResourceAccess    map[string]clientRoleMapping `json:"resource_access"`
}

type clientRoleMapping struct {
	Roles []string `json:"roles"`
}

// userProfile represents the normalized attributes consumed by Mattermost provisioning logic.
type userProfile struct {
	Subject       string
	Email         string
	EmailVerified bool
	Username      string
	FirstName     string
	LastName      string
	DisplayName   string
	SystemAdmin   bool
}

func buildUserProfile(claims *idTokenClaims, clientID string) userProfile {
	if claims == nil {
		return userProfile{}
	}

	profile := userProfile{
		Subject:       claims.Subject,
		Email:         strings.ToLower(claims.Email),
		EmailVerified: claims.EmailVerified,
		Username:      claims.PreferredUsername,
		FirstName:     claims.GivenName,
		LastName:      claims.FamilyName,
		DisplayName:   claims.Name,
	}

	if profile.Username == "" {
		profile.Username = profile.Email
	}
	if profile.Username == "" {
		profile.Username = claims.Subject
	}

	if hasClientRole(claims, clientID, clientAdminRole) {
		profile.SystemAdmin = true
	}

	return profile
}

func hasClientRole(claims *idTokenClaims, clientID, role string) bool {
	if claims == nil || clientID == "" || role == "" {
		return false
	}

	access, ok := claims.ResourceAccess[clientID]
	if !ok {
		return false
	}

	for _, candidate := range access.Roles {
		if candidate == role {
			return true
		}
	}

	return false
}
