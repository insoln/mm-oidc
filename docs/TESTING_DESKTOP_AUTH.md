# Desktop App Authentication Testing Guide

This document explains how to test and verify the desktop app authentication flow.

## Overview

The desktop app authentication flow uses:
1. User-Agent detection to identify desktop apps (Electron/Mattermost)
2. HTML page with `window.open()` instead of HTTP redirect for /login
3. Completion page instead of homepage redirect for /callback
4. IsDesktopApp flag persisted in auth session

## Unit Tests

Run all unit tests:
```bash
cd /home/runner/work/mm-oidc/mm-oidc
make server-test
```

Run desktop flow test specifically:
```bash
cd server
go test -v -run TestDesktopAppFullFlow
```

This test verifies:
- ✅ User-Agent detection works for Electron/Mattermost
- ✅ IsDesktopApp flag persists through JSON serialization
- ✅ Completion page renders correctly (HTML, not redirect)
- ✅ No Location header in completion page response

## Manual Testing with curl

### Step 1: Test /login endpoint

Test with desktop User-Agent:
```bash
curl -v "http://localhost:8065/plugins/com.mm.oidc/login?redirect_to=/" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.7339.249 Electron/38.7.2 Safari/537.36 Mattermost/6.0.2"
```

Expected response:
- HTTP Status: `200 OK` (NOT `302 Found`)
- Content-Type: `text/html; charset=utf-8`
- Body contains: `window.open`
- Body contains: `Opening Browser`
- NO `Location:` header

Compare with web browser User-Agent:
```bash
curl -v "http://localhost:8065/plugins/com.mm.oidc/login?redirect_to=/" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
```

Expected response:
- HTTP Status: `302 Found`
- `Location:` header pointing to Keycloak authorization endpoint

### Step 2: Check Mattermost logs

Enable debug logging and check that flags are set correctly:

```bash
docker logs deploy-mattermost-1 2>&1 | grep -i "login request received\|auth session saved\|callback received"
```

Expected log entries:
```
login request received is_desktop=true user_agent=...Electron...Mattermost...
auth session saved state=... is_desktop_app=true
callback received is_desktop_app=true user_agent=...Chrome...
```

### Step 3: Test full authentication flow

1. Desktop app requests /login with Electron User-Agent
2. Page renders with window.open(keycloak_url)
3. Desktop app intercepts window.open and opens system browser
4. User authenticates in browser
5. Browser redirects to /callback
6. Check response from /callback:

```bash
# This would come from the browser after authentication
# Note: You need actual state and code from OIDC provider
curl -v "http://localhost:8065/plugins/com.mm.oidc/callback?state=<state>&code=<code>" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
```

Expected response (if IsDesktopApp=true in session):
- HTTP Status: `200 OK` (NOT `302 Found`)
- Content-Type: `text/html; charset=utf-8`
- Body contains: `Authentication Complete`
- Body contains: `close this browser window`
- Body contains: `window.close()`
- NO `Location:` header

Expected response (if IsDesktopApp=false in session):
- HTTP Status: `302 Found`
- `Location:` header pointing to Mattermost homepage

## Troubleshooting

### Issue: Callback still redirects to homepage

**Cause:** Old auth sessions created before code deployment don't have IsDesktopApp field.

**Solution:** 
1. Clear old sessions (restart Mattermost or wait for TTL expiry)
2. Start fresh login flow
3. Check logs to verify IsDesktopApp=true is set

### Issue: User-Agent not detected

**Check:** 
```bash
cd server
go test -v -run TestIsDesktopOrMobileApp
```

Verify that your User-Agent contains:
- "Electron" OR
- "Mattermost/" pattern (e.g., "Mattermost/6.0.2")

### Issue: window.open() not working

**Desktop app requirements:**
```javascript
// In Electron app's main process:
webContents.setWindowOpenHandler(({ url }) => {
  if (url.includes('oauth') || url.includes('authorize')) {
    shell.openExternal(url);
    return { action: 'deny' };
  }
  return { action: 'deny' };
});
```

## Test Results

As of commit 087a09b:

✅ Unit tests: PASS (0.008s)
✅ Desktop flow test: PASS
  - User-Agent detection works
  - IsDesktopApp flag persists through JSON
  - Completion page renders correctly
  - No redirect occurs

✅ Code compilation: SUCCESS
✅ JSON serialization: VERIFIED
✅ Function signatures: CORRECT

## Next Steps

1. Deploy plugin to running Mattermost instance
2. Run manual curl tests against live server
3. Test with actual Mattermost Desktop application
4. Verify logs show correct IsDesktopApp flag propagation
