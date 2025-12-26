# Desktop and Mobile App Authentication

This document describes how desktop and mobile applications can authenticate users via the Mattermost OIDC Plugin using an external browser flow.

## Overview

The plugin provides a special authentication flow for desktop and mobile applications that cannot use embedded webviews for security reasons. Instead, the flow opens the user's default browser for authentication and returns control to the app via a custom protocol handler.

This implementation matches the native Mattermost OAuth flow used in Mattermost Desktop v5.x+ and Mobile v2.x+.

## Flow Diagram

```
┌──────────────┐                ┌──────────────┐                ┌──────────────┐
│   Desktop    │                │   Browser    │                │   Keycloak   │
│     App      │                │              │                │   (OIDC)     │
└──────┬───────┘                └──────┬───────┘                └──────┬───────┘
       │                               │                               │
       │ 1. Open browser               │                               │
       ├──────────────────────────────>│                               │
       │ /login/mobile?redirect_to=... │                               │
       │                               │                               │
       │                               │ 2. Redirect to OIDC provider  │
       │                               ├──────────────────────────────>│
       │                               │                               │
       │                               │ 3. User authenticates         │
       │                               │<──────────────────────────────┤
       │                               │                               │
       │                               │ 4. Auth code + state          │
       │                               │<──────────────────────────────┤
       │                               │                               │
       │                               │ 5. POST to /callback/mobile   │
       │                               ├──────────────────────────────>│
       │                               │    (exchange code for tokens) │
       │                               │                               │
       │                               │ 6. HTML page with redirect    │
       │                               │<───────────────────────────────
       │                               │    mattermost://auth?token=..  │
       │                               │                               │
       │ 7. Protocol handler invoked   │                               │
       │<──────────────────────────────┤                               │
       │   mattermost://auth?token=... │                               │
       │                               │                               │
       │ 8. Session established        │                               │
       │                               │                               │
```

## Implementation Guide

### 1. Desktop App Requirements

Your desktop application must:

1. **Register a custom protocol handler** (e.g., `mattermost://` or `mattermostdesktop://`)
   - On Windows: Register via the Windows Registry
   - On macOS: Declare in `Info.plist`
   - On Linux: Register via `.desktop` file

2. **Open the system browser** to initiate authentication

3. **Handle protocol callbacks** to receive authentication tokens

### 2. Authentication Endpoint

**Endpoint:** `/plugins/com.mm.oidc/login/mobile`

**Method:** `GET`

**Query Parameters:**
- `redirect_to` (required): The custom protocol URL where the app expects to receive tokens
  - Must use a custom protocol scheme (e.g., `mattermost://`, `mattermostdesktop://`)
  - Cannot use `http://` or `https://`

**Example:**
```
https://mattermost.example.com/plugins/com.mm.oidc/login/mobile?redirect_to=mattermost://auth/complete
```

### 3. Desktop App Implementation Example

#### JavaScript/Electron Example

```javascript
const { shell } = require('electron');
const url = require('url');

// Step 1: Generate and store a random state for security
const state = generateRandomString(32);
sessionStorage.setItem('oauth_state', state);

// Step 2: Construct the login URL
const mattermostUrl = 'https://mattermost.example.com';
const pluginId = 'com.mm.oidc';
const redirectTo = encodeURIComponent('mattermost://auth/complete');
const loginUrl = `${mattermostUrl}/plugins/${pluginId}/login/mobile?redirect_to=${redirectTo}`;

// Step 3: Open the default browser
shell.openExternal(loginUrl);

// Step 4: Register protocol handler (in main process)
app.setAsDefaultProtocolClient('mattermost');

// Step 5: Handle the protocol callback
app.on('open-url', (event, url) => {
  event.preventDefault();
  
  const parsedUrl = new URL(url);
  const token = parsedUrl.searchParams.get('MMAUTHTOKEN');
  const csrf = parsedUrl.searchParams.get('MMCSRF');
  
  if (token && csrf) {
    // Store tokens and establish session
    sessionStorage.setItem('authToken', token);
    sessionStorage.setItem('csrfToken', csrf);
    
    // Continue with application logic
    console.log('Authentication successful');
  }
});
```

#### Python/PyQt Example

```python
import webbrowser
from urllib.parse import urlparse, parse_qs
from PyQt6.QtCore import QUrl
from PyQt6.QtGui import QDesktopServices

class AuthHandler:
    def __init__(self, mattermost_url):
        self.mattermost_url = mattermost_url
        self.plugin_id = 'com.mm.oidc'
    
    def start_authentication(self):
        # Construct login URL
        redirect_to = 'mattermostapp://auth/complete'
        login_url = (
            f'{self.mattermost_url}/plugins/{self.plugin_id}/login/mobile'
            f'?redirect_to={redirect_to}'
        )
        
        # Open in default browser
        webbrowser.open(login_url)
    
    def handle_callback(self, url_string):
        """Called when protocol handler receives callback"""
        url = urlparse(url_string)
        params = parse_qs(url.query)
        
        token = params.get('MMAUTHTOKEN', [None])[0]
        csrf = params.get('MMCSRF', [None])[0]
        
        if token and csrf:
            # Store tokens and establish session
            self.session_token = token
            self.csrf_token = csrf
            print('Authentication successful')
            return True
        
        return False
```

### 4. Callback Parameters

After successful authentication, the browser will invoke your custom protocol handler with the following query parameters:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `MMAUTHTOKEN` | Mattermost session token | `abcdef123456...` |
| `MMCSRF` | CSRF token for the session | `xyz789...` |

**Example callback URL:**
```
mattermost://auth/complete?MMAUTHTOKEN=abcdef123456&MMCSRF=xyz789
```

### 5. Using the Tokens

Once your app receives the tokens via the protocol handler:

1. **Store the tokens securely** in your application's secure storage
2. **Include the session token** in API requests:
   - As a cookie: `MMAUTHTOKEN=<token>`
   - As a header: `Authorization: Bearer <token>`
   - As a header: `X-CSRF-Token: <csrf-token>`

3. **Make API calls** to Mattermost using the token:

```javascript
// Example API request
fetch('https://mattermost.example.com/api/v4/users/me', {
  headers: {
    'Authorization': `Bearer ${sessionToken}`,
    'X-CSRF-Token': csrfToken,
  }
})
.then(response => response.json())
.then(user => console.log('Current user:', user));
```

## Configuration

### OIDC Provider Setup

In your OIDC provider (e.g., Keycloak), configure the client with **two redirect URIs**:

1. **Web redirect URI:**
   ```
   https://mattermost.example.com/plugins/com.mm.oidc/callback
   ```

2. **Mobile/Desktop redirect URI:**
   ```
   https://mattermost.example.com/plugins/com.mm.oidc/callback/mobile
   ```

### Keycloak Example

1. Navigate to your realm in Keycloak Admin Console
2. Go to **Clients** → Select your client
3. Add both redirect URIs to **Valid Redirect URIs**:
   ```
   https://mattermost.example.com/plugins/com.mm.oidc/callback
   https://mattermost.example.com/plugins/com.mm.oidc/callback/mobile
   ```
4. Save changes

## Security Considerations

### Custom Protocol Validation

The plugin validates that `redirect_to` uses a custom protocol scheme and rejects standard HTTP/HTTPS URLs to prevent open redirect vulnerabilities.

Valid schemes:
- ✅ `mattermost://`
- ✅ `mattermostdesktop://`
- ✅ `myapp://`

Invalid schemes:
- ❌ `http://`
- ❌ `https://`
- ❌ Empty or missing

### State Management

The plugin automatically manages OIDC state/nonce parameters and PKCE flow. Your desktop app doesn't need to implement these - they're handled server-side.

### Token Security

- **Session tokens** are short-lived and tied to the user's Mattermost session
- **CSRF tokens** protect against cross-site request forgery
- Always store tokens in secure storage (e.g., OS keychain)
- Never log or expose tokens in plain text

## Troubleshooting

### Browser shows "Authentication Complete" but app doesn't respond

**Cause:** Protocol handler not registered or not responding

**Solution:**
1. Verify protocol handler registration on the OS
2. Check application logs for protocol handler events
3. Test protocol handler manually: paste `mattermost://test` in browser

### Error: "Invalid redirect URL"

**Cause:** `redirect_to` parameter uses invalid scheme

**Solution:**
- Ensure `redirect_to` uses a custom protocol (not http/https)
- URL-encode the `redirect_to` parameter
- Example: `?redirect_to=mattermost%3A%2F%2Fauth%2Fcomplete`

### Error: "Missing redirect URL"

**Cause:** `redirect_to` parameter not provided

**Solution:**
- Always include `redirect_to` query parameter
- Example: `/login/mobile?redirect_to=mattermost://auth`

### Tokens not received in callback

**Cause:** Protocol handler not capturing URL parameters

**Solution:**
- Verify protocol handler captures full URL including query parameters
- Log the received URL to debug parameter parsing
- Check URL encoding/decoding

## Testing

### Manual Testing

1. **Register protocol handler** on your development machine
2. **Open browser** to the mobile login URL:
   ```
   https://your-mattermost/plugins/com.mm.oidc/login/mobile?redirect_to=mattermost://test
   ```
3. **Authenticate** through the OIDC provider
4. **Verify** that your app receives the protocol callback with tokens

### Automated Testing

Create integration tests that:
1. Mock the protocol handler
2. Simulate the authentication flow
3. Verify token reception and storage
4. Test error scenarios (invalid redirect_to, expired sessions, etc.)

## References

- [Mattermost Desktop Repository](https://github.com/mattermost/desktop)
- [Mattermost Mobile Repository](https://github.com/mattermost/mattermost-mobile)
- [OAuth 2.0 for Native Apps (RFC 8252)](https://datatracker.ietf.org/doc/html/rfc8252)
- [PKCE (RFC 7636)](https://datatracker.ietf.org/doc/html/rfc7636)

## Support

For issues or questions:
1. Check existing [GitHub Issues](https://github.com/insoln/mm-oidc/issues)
2. Review [Architecture Documentation](ARCHITECTURE.md)
3. Open a new issue with:
   - Desktop/mobile app details (platform, version)
   - Plugin version
   - OIDC provider (Keycloak, etc.)
   - Error messages or logs (redact sensitive data)
