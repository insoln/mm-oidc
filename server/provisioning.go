package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

var usernameSanitizeRegex = regexp.MustCompile(`[^a-z0-9._-]+`)

// provisionUser ensures a Mattermost user exists for the supplied OIDC profile.
func (p *Plugin) provisionUser(profile userProfile) (*model.User, error) {
	if profile.Subject == "" {
		return nil, fmt.Errorf("profile subject is required")
	}
	if profile.Email == "" {
		return nil, fmt.Errorf("profile email is required")
	}

	if user, err := p.getUserBySubject(profile.Subject); err != nil {
		return nil, err
	} else if user != nil {
		return p.syncAndLinkUser(user, profile)
	}

	if user, err := p.getUserByEmail(profile.Email); err != nil {
		return nil, err
	} else if user != nil {
		return p.syncAndLinkUser(user, profile)
	}

	return p.createUser(profile)
}

func (p *Plugin) getUserByEmail(email string) (*model.User, error) {
	user, appErr := p.API.GetUserByEmail(email)
	if appErr != nil {
		if appErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email: %w", appErr)
	}

	return user, nil
}

func (p *Plugin) createUser(profile userProfile) (*model.User, error) {
	username, err := p.generateUniqueUsername(profile.Username)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:         profile.Email,
		Username:      username,
		FirstName:     profile.FirstName,
		LastName:      profile.LastName,
		Nickname:      profile.DisplayName,
		AuthService:   pluginAuthService,
		AuthData:      stringPtr(profile.Subject),
		EmailVerified: profile.EmailVerified,
		Password:      model.NewId(),
	}

	created, appErr := p.API.CreateUser(user)
	if appErr != nil {
		return nil, fmt.Errorf("create user: %w", appErr)
	}

	if err := p.ensureSystemRoles(created, profile.SystemAdmin); err != nil {
		return nil, err
	}

	if err := p.saveUserMapping(profile.Subject, created.Id); err != nil {
		return nil, err
	}

	return created, nil
}

func (p *Plugin) syncAndLinkUser(user *model.User, profile userProfile) (*model.User, error) {
	updated := false
	resetPassword := false

	if user.AuthService != pluginAuthService {
		user.AuthService = pluginAuthService
		resetPassword = true
		updated = true
	}
	if user.Password != "" && (resetPassword || user.AuthService == pluginAuthService) {
		// Mattermost forbids storing both password hashes and external auth data; clear the
		// password so local accounts can be converted to OIDC without manual cleanup.
		user.Password = ""
		user.LastPasswordUpdate = 0
		updated = true
	}
	if user.AuthData == nil || *user.AuthData != profile.Subject {
		user.AuthData = stringPtr(profile.Subject)
		updated = true
	}
	if profile.FirstName != "" && user.FirstName != profile.FirstName {
		user.FirstName = profile.FirstName
		updated = true
	}
	if profile.LastName != "" && user.LastName != profile.LastName {
		user.LastName = profile.LastName
		updated = true
	}
	if profile.DisplayName != "" && user.Nickname != profile.DisplayName {
		user.Nickname = profile.DisplayName
		updated = true
	}
	if profile.EmailVerified && !user.EmailVerified {
		user.EmailVerified = true
		updated = true
	}
	if updated {
		saved, appErr := p.API.UpdateUser(user)
		if appErr != nil {
			return nil, fmt.Errorf("update user: %w", appErr)
		}
		user = saved
	}

	if err := p.ensureSystemRoles(user, profile.SystemAdmin); err != nil {
		return nil, err
	}

	if err := p.saveUserMapping(profile.Subject, user.Id); err != nil {
		return nil, err
	}

	return user, nil
}

func sanitizeUsername(input string) string {
	cleaned := strings.ToLower(strings.TrimSpace(input))
	cleaned = strings.ReplaceAll(cleaned, " ", "-")
	cleaned = usernameSanitizeRegex.ReplaceAllString(cleaned, "")
	cleaned = strings.Trim(cleaned, ".-_")
	return cleaned
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func applySystemRoles(user *model.User, grantAdmin bool) bool {
	if user == nil {
		return false
	}

	currentRoles := strings.Fields(strings.TrimSpace(user.Roles))
	roleSet := make(map[string]bool, len(currentRoles)+2)
	for _, role := range currentRoles {
		if role != "" {
			roleSet[role] = true
		}
	}

	changed := false
	if !roleSet[model.SystemUserRoleId] {
		roleSet[model.SystemUserRoleId] = true
		changed = true
	}

	if grantAdmin {
		if !roleSet[model.SystemAdminRoleId] {
			roleSet[model.SystemAdminRoleId] = true
			changed = true
		}
	} else {
		if roleSet[model.SystemAdminRoleId] {
			delete(roleSet, model.SystemAdminRoleId)
			changed = true
		}
	}

	if !changed {
		return false
	}

	ordered := make([]string, 0, len(roleSet))
	seen := make(map[string]bool, len(roleSet))
	for _, role := range currentRoles {
		if roleSet[role] && !seen[role] {
			ordered = append(ordered, role)
			seen[role] = true
		}
	}
	for _, role := range []string{model.SystemUserRoleId, model.SystemAdminRoleId} {
		if roleSet[role] && !seen[role] {
			ordered = append(ordered, role)
			seen[role] = true
		}
	}
	for role := range roleSet {
		if !seen[role] {
			ordered = append(ordered, role)
			seen[role] = true
		}
	}

	if len(ordered) == 0 {
		ordered = []string{model.SystemUserRoleId}
	}

	user.Roles = strings.Join(ordered, " ")
	return true
}

func (p *Plugin) ensureSystemRoles(user *model.User, grantAdmin bool) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	snapshot := *user
	if !applySystemRoles(&snapshot, grantAdmin) {
		p.API.LogDebug("system roles already in desired state", "user_id", user.Id, "grant_admin", grantAdmin, "roles", user.Roles)
		return nil
	}

	updated, appErr := p.API.UpdateUserRoles(user.Id, snapshot.Roles)
	if appErr != nil {
		return fmt.Errorf("update user roles: %w", appErr)
	}

	if updated != nil {
		user.Roles = updated.Roles
	} else {
		user.Roles = snapshot.Roles
	}

	p.API.LogDebug("system roles updated", "user_id", user.Id, "grant_admin", grantAdmin, "roles", user.Roles)
	return nil
}

func (p *Plugin) generateUniqueUsername(preferred string) (string, error) {
	base := sanitizeUsername(preferred)
	if base == "" {
		base = fmt.Sprintf("oidc-%s", strings.ToLower(model.NewId()[:8]))
	}

	candidate := base
	suffix := 1
	for {
		if _, appErr := p.API.GetUserByUsername(candidate); appErr != nil {
			if appErr.StatusCode == http.StatusNotFound {
				return candidate, nil
			}
			return "", fmt.Errorf("check username availability: %w", appErr)
		}

		candidate = fmt.Sprintf("%s-%d", base, suffix)
		suffix++
		if suffix > 50 {
			return fmt.Sprintf("oidc-%s", strings.ToLower(model.NewId()[:12])), nil
		}
	}
}

func (p *Plugin) getUserBySubject(subject string) (*model.User, error) {
	if subject == "" {
		return nil, nil
	}

	userID, err := p.loadUserMapping(subject)
	if err != nil {
		return nil, err
	}

	if userID == "" {
		return nil, nil
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		if appErr.StatusCode == http.StatusNotFound {
			_ = p.deleteUserMapping(subject)
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", appErr)
	}

	return user, nil
}

func (p *Plugin) saveUserMapping(subject, userID string) error {
	if subject == "" || userID == "" {
		return fmt.Errorf("invalid mapping input")
	}

	if appErr := p.API.KVSet(userLinkKey(subject), []byte(userID)); appErr != nil {
		return fmt.Errorf("persist user mapping: %w", appErr)
	}

	return nil
}

func (p *Plugin) loadUserMapping(subject string) (string, error) {
	if subject == "" {
		return "", nil
	}

	data, appErr := p.API.KVGet(userLinkKey(subject))
	if appErr != nil {
		return "", fmt.Errorf("load user mapping: %w", appErr)
	}

	return string(data), nil
}

func (p *Plugin) deleteUserMapping(subject string) error {
	if subject == "" {
		return nil
	}

	if appErr := p.API.KVDelete(userLinkKey(subject)); appErr != nil {
		return fmt.Errorf("delete user mapping: %w", appErr)
	}

	return nil
}

func userLinkKey(subject string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(subject))
	return "oidc:user:" + encoded
}
