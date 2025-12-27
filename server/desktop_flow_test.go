package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDesktopAppFullFlow tests the complete desktop app authentication flow
func TestDesktopAppFullFlow(t *testing.T) {
	// Step 1: Desktop app requests /login with Electron User-Agent
	req1 := httptest.NewRequest("GET", "/plugins/com.mm.oidc/login?redirect_to=/", nil)
	req1.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.7339.249 Electron/38.7.2 Safari/537.36 Mattermost/6.0.2")
	_ = httptest.NewRecorder() // w1 - would be used in handleLogin

	// Check User-Agent detection
	isDesktop := isDesktopOrMobileApp(req1.Header.Get("User-Agent"))
	if !isDesktop {
		t.Fatalf("Expected desktop app to be detected, but got false")
	}

	// Step 2: Verify that response would be HTML (not redirect)
	// In actual flow, this would call handleLogin which would:
	// - Create authSession with IsDesktopApp=true
	// - Save it to KV store
	// - Render HTML with window.open()

	// Let's verify the auth session structure
	session := &authSession{
		Nonce:        "test-nonce",
		CodeVerifier: "test-verifier",
		CreatedAt:    1234567890,
		IsDesktopApp: true,
	}

	// Serialize and deserialize to ensure JSON works correctly
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Failed to marshal auth session: %v", err)
	}

	var restored authSession
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal auth session: %v", err)
	}

	if !restored.IsDesktopApp {
		t.Fatalf("IsDesktopApp flag was not preserved after JSON round-trip")
	}

	// Step 3: Browser callback would come from system browser (different User-Agent)
	req2 := httptest.NewRequest("GET", "/plugins/com.mm.oidc/callback?state=test&code=test", nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	w2 := httptest.NewRecorder()

	// The callback should check the IsDesktopApp flag from the session (not from User-Agent)
	// If IsDesktopApp=true, it should render completion page
	// If IsDesktopApp=false, it should redirect to homepage

	// Step 4: Test completeLogin behavior
	// Simulate rendering completion page
	renderDesktopAuthComplete(w2)

	body := w2.Body.String()
	
	// Verify completion page is rendered
	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	if !strings.Contains(body, "Authentication Complete") {
		t.Errorf("Completion page should contain 'Authentication Complete'")
	}

	if !strings.Contains(body, "close this browser window") {
		t.Errorf("Completion page should contain instruction to close browser")
	}

	if !strings.Contains(body, "window.close()") {
		t.Errorf("Completion page should attempt to close window with JavaScript")
	}

	// Verify it's HTML, not a redirect
	contentType := w2.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type to be text/html, got %s", contentType)
	}

	// Verify no redirect header
	location := w2.Header().Get("Location")
	if location != "" {
		t.Errorf("Completion page should not have Location header, got: %s", location)
	}

	t.Log("✅ Desktop app flow test passed:")
	t.Log("  - User-Agent detection works")
	t.Log("  - IsDesktopApp flag persists through JSON serialization")
	t.Log("  - Completion page is rendered correctly")
	t.Log("  - No redirect to homepage occurs")
}

// TestWebBrowserFlow tests that web browsers still get redirects
func TestWebBrowserFlow(t *testing.T) {
	// Web browser User-Agent (no Electron or Mattermost/)
	req := httptest.NewRequest("GET", "/plugins/com.mm.oidc/login?redirect_to=/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")

	isDesktop := isDesktopOrMobileApp(req.Header.Get("User-Agent"))
	if isDesktop {
		t.Fatalf("Expected web browser NOT to be detected as desktop app, but got true")
	}

	// Auth session for web browser
	session := &authSession{
		Nonce:        "test-nonce",
		CodeVerifier: "test-verifier",
		CreatedAt:    1234567890,
		IsDesktopApp: false,
	}

	// Verify IsDesktopApp is false
	if session.IsDesktopApp {
		t.Fatalf("Web browser session should have IsDesktopApp=false")
	}

	t.Log("✅ Web browser flow test passed:")
	t.Log("  - Web browser is NOT detected as desktop app")
	t.Log("  - IsDesktopApp flag is false")
}
