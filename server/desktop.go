package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	desktopTokenTTL        = 3 * time.Minute
	desktopTokenBytes      = 48
	desktopTokenKeyPrefix  = "desktop_token:"
	desktopSessionKeyPrefix = "desktop_session:"
)

// desktopTokenRecord stores the mapping from desktop token to user ID
type desktopTokenRecord struct {
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
}

// generateDesktopToken creates a random token for desktop app authentication
func generateDesktopToken() (string, error) {
	b := make([]byte, desktopTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate desktop token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// saveDesktopToken stores the desktop token mapping
func (p *Plugin) saveDesktopToken(token string, userID string) error {
	record := &desktopTokenRecord{
		UserID:    userID,
		CreatedAt: time.Now().Unix(),
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal desktop token record: %w", err)
	}

	ttlSeconds := int64(desktopTokenTTL.Seconds())
	if appErr := p.API.KVSetWithExpiry(desktopTokenKey(token), payload, ttlSeconds); appErr != nil {
		return fmt.Errorf("persist desktop token: %w", appErr)
	}

	return nil
}

// consumeDesktopToken retrieves and deletes the desktop token mapping
func (p *Plugin) consumeDesktopToken(token string) (string, error) {
	data, appErr := p.API.KVGet(desktopTokenKey(token))
	if appErr != nil {
		return "", fmt.Errorf("load desktop token: %w", appErr)
	}
	if data == nil {
		return "", fmt.Errorf("desktop token not found or expired")
	}

	// Delete the token immediately after reading
	if err := p.API.KVDelete(desktopTokenKey(token)); err != nil {
		p.API.LogWarn("failed to delete desktop token", "token_prefix", token[:8], "error", err.Error())
	}

	var record desktopTokenRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return "", fmt.Errorf("decode desktop token record: %w", err)
	}

	// Check if token has expired
	if time.Since(time.Unix(record.CreatedAt, 0)) > desktopTokenTTL {
		return "", fmt.Errorf("desktop token expired")
	}

	return record.UserID, nil
}

func desktopTokenKey(token string) string {
	return desktopTokenKeyPrefix + token
}

// handleLoginDesktop renders the desktop redirect page with tokens
func (p *Plugin) handleLoginDesktop(w http.ResponseWriter, r *http.Request) {
	clientToken := r.URL.Query().Get("client_token")
	serverToken := r.URL.Query().Get("server_token")
	redirectTo := r.URL.Query().Get("redirect_to")
	isDesktopDev := r.URL.Query().Get("isDesktopDev") == "true"

	if clientToken == "" || serverToken == "" {
		p.writeFriendlyError(w, 400, "Invalid request", "Missing required tokens for desktop login.")
		return
	}

	// Build the desktop URL scheme redirect
	desktopURL := "mattermost://callback"
	if isDesktopDev {
		desktopURL = "mattermost-dev://callback"
	}

	queryParams := fmt.Sprintf("?client_token=%s&server_token=%s", clientToken, serverToken)
	if redirectTo != "" {
		queryParams += fmt.Sprintf("&redirect_to=%s", redirectTo)
	}

	fullRedirectURL := desktopURL + queryParams

	// Render HTML page that triggers desktop app
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8" />
	<title>Launching Mattermost Desktop</title>
	<meta http-equiv="refresh" content="0; url=%s" />
	<style>
		body { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; margin: 0; padding: 2rem; background: #0f172a; color: #f1f5f9; text-align: center; }
		.card { max-width: 560px; margin: 2rem auto; background: rgba(15,23,42,0.85); border-radius: 16px; padding: 2rem; }
		h1 { margin-top: 0; font-size: 1.5rem; }
		p { line-height: 1.5; margin: 1rem 0; }
		.spinner { display: inline-block; width: 40px; height: 40px; margin: 1.5rem 0; border: 4px solid rgba(56,189,248,0.3); border-top-color: #38bdf8; border-radius: 50%%; animation: spin 1s linear infinite; }
		@keyframes spin { to { transform: rotate(360deg); } }
		a { color: #38bdf8; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<main class="card">
		<h1>Launching Mattermost Desktop</h1>
		<div class="spinner"></div>
		<p>Redirecting to your desktop app...</p>
		<p>If the app doesn't open automatically, <a href="%s">click here</a>.</p>
	</main>
</body>
</html>`, fullRedirectURL, fullRedirectURL)
}

// handleLoginDesktopToken handles the API endpoint for desktop token login
func (p *Plugin) handleLoginDesktopToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token := payload["token"]
	deviceID := payload["device_id"]

	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Retrieve and consume the desktop token
	userID, err := p.consumeDesktopToken(token)
	if err != nil {
		p.API.LogError("failed to validate desktop token", "error", err.Error())
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Get the user
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		p.API.LogError("failed to get user for desktop token", "user_id", userID, "error", appErr.Error())
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Create a session for the user
	session := &model.Session{
		UserId:   user.Id,
		Roles:    strings.TrimSpace(user.Roles),
		DeviceId: deviceID,
		IsOAuth:  true,
	}
	if session.Roles == "" {
		session.Roles = model.SystemUserRoleId
	}
	session.PreSave()
	session.GenerateCSRF()
	session.Id = ""

	createdSession, appErr := p.API.CreateSession(session)
	if appErr != nil {
		p.API.LogError("failed to create session for desktop token", "user_id", userID, "error", appErr.Error())
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Return the user data with session token
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Token", createdSession.Token)
	
	if err := json.NewEncoder(w).Encode(user); err != nil {
		p.API.LogError("failed to encode user response", "error", err.Error())
	}

	p.API.LogDebug("desktop token login successful", "user_id", userID)
}
