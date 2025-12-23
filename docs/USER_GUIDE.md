# Mattermost OIDC Plugin – User Guide

This guide walks system administrators through installing, configuring, and validating the OIDC plugin across two deployment scenarios:

1. Adding the plugin to an already running Mattermost cluster.
2. Fronting Mattermost with the provided proxy container (or an equivalent Ingress/Nginx deployment) so `/login` automatically reroutes users into the plugin flow.

For developer workflows (local Docker stack, automated tests, packaging from source), refer to [docs/DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md).

Each section contains prerequisites, ordered steps, verification tips, and explicit limitations so you can decide which deployment mode matches your environment.

---

## 1. Install the plugin into an existing Mattermost deployment

### Prerequisites

- Mattermost Server **v9.0+** with plugin uploads enabled.
- Administrator access to your Identity Provider (Keycloak v25.x is the reference implementation).
- HTTPS endpoints for both Mattermost and the IdP (dev-only stacks can temporarily allow HTTP via the `Allow insecure issuer` toggle).

### Step-by-step

1. **Download the plugin package**
  - Preferred: grab the latest signed archive from [GitHub Releases](https://github.com/insoln/mm-oidc/releases) (`mm-oidc.tar.gz`).
  - Need to build from source? Follow the packaging steps in [docs/DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md).
2. **Provision (or reuse) a confidential client in your IdP**
  - Follow the dedicated Keycloak instructions in [KEYCLOAK_SETUP.md](KEYCLOAK_SETUP.md) or replicate them for your IdP of choice.
   - Required mappers: `preferred_username`, `given_name`, `family_name`, `full_name`, `email`, and (optionally) a client-role mapper that emits `system_admin`.
3. **Upload and enable the plugin**
   - Sign in to Mattermost as a system admin.
   - Navigate to **System Console → Plugin Management → Management**.
   - Enable **Plugin Uploads** if the setting is disabled, then upload `mm-oidc.tar.gz`.
   - After the upload finishes, click **Enable** next to `Mattermost OIDC (com.mm.oidc)`.
4. **Configure plugin settings**
   - Open **System Console → Plugins → Mattermost OIDC** and populate:
     - `Issuer URL`: `https://<keycloak-host>/realms/<realm>`
     - `Client ID` and `Client Secret`: from your IdP client
     - `Redirect URL`: `https://<mattermost-host>/plugins/com.mm.oidc/callback`
     - `Scopes`: `openid profile email` (append `roles` if you mapped them)
     - Toggle `Allow insecure issuer` **off** outside disposable labs.
   - Click **Save** and use **Start Login** to verify the round-trip.
5. **Roll out to users**
  - Link the plugin landing page (`/plugins/com.mm.oidc/`) from your login experience or add a reverse-proxy rule (see section 2) so the plugin drives every interactive login.

### Verification

1. Sign out of Mattermost or open a private browsing window.
2. Navigate to `https://<mattermost-host>/plugins/com.mm.oidc/` and click **Start Login**.
3. Complete authentication with your Identity Provider.
4. Confirm the Mattermost UI shows the expected user details and that the `MMAUTHTOKEN` cookie is present in your browser.

Automated regression scripts live in [docs/DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) if you need repeatable validation for CI.

### Known limitations in vanilla Mattermost

- The plugin **cannot override** the stock `/login` and `/logout` routes; only `/plugins/com.mm.oidc/` and `/plugins/com.mm.oidc/login` launch the flow.
- Mobile apps and legacy password clients must continue using the built-in authentication methods until you add a proxy rule or custom UI entry point.
- Server metrics/logging already redact secrets, but Mattermost still displays raw IdP URLs inside the System Console; secure that interface appropriately.

---

## 2. Deploy the proxy container (Docker Compose or Kubernetes)

Redirecting `/login` to `/plugins/com.mm.oidc/login` requires a front proxy that can inspect cookies and selectively rewrite requests. The repository ships with a ready-to-use Nginx container that you can copy into your own stack.

### 2.1 Docker Compose

1. **Import the service definition**
   - Reuse the `mattermost-proxy` service block from [`deploy/docker-compose.dev.yml`](deploy/docker-compose.dev.yml) and adjust the `ports`, `MM_SITE_URL`, and hostname values to match your environment.
   - Mount the maintained config [deploy/mattermost-proxy/nginx.conf](deploy/mattermost-proxy/nginx.conf) via a bind mount or ConfigMap.
2. **Wire the network**
   - Place the proxy and Mattermost containers on the same Docker network.
   - Update Mattermost’s `SiteURL` to `http(s)://<proxy-host>` so cookies and the plugin’s redirect metadata stay consistent.
3. **Expose the proxy**
   - Publish the proxy’s port(s) to your load balancer or ingress; the upstream Mattermost port should remain internal.
4. **Validate**
  - In a private browser window, visit the proxy host and confirm unauthenticated requests redirect to `/plugins/com.mm.oidc/login`.
  - Hit `/api/v4/users/login` with `curl -H 'Accept: application/json'` and verify the response is not a redirect.
  - Open `/plugins/com.mm.oidc/callback` directly to ensure it stays reachable (no redirect loops) and that WebSocket upgrades still succeed via `/api/v4/websocket`.

### 2.2 Kubernetes (example)

1. **ConfigMap** – create a ConfigMap with the Nginx template:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mm-oidc-proxy-config
  namespace: mattermost
data:
  nginx.conf: |
    # copy the contents of deploy/mattermost-proxy/nginx.conf
```

2. **Deployment** – run the proxy as a sidecar fronting Mattermost:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mattermost-proxy
  namespace: mattermost
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mattermost-proxy
  template:
    metadata:
      labels:
        app: mattermost-proxy
    spec:
      containers:
        - name: nginx
          image: nginx:1.25-alpine
          ports:
            - containerPort: 8787
          volumeMounts:
            - name: proxy-config
              mountPath: /etc/nginx/conf.d/default.conf
              subPath: nginx.conf
          env:
            - name: MATTERMOST_UPSTREAM
              value: "http://mattermost:8065"
      volumes:
        - name: proxy-config
          configMap:
            name: mm-oidc-proxy-config
```

3. **Ingress / Service** – expose the proxy via your preferred ingress controller, ensuring sticky sessions and TLS termination live at the edge.
4. **Plugin config** – set Mattermost’s `SiteURL` to the proxy host and keep the plugin’s redirect URL aligned (e.g., `https://chat.example.com/plugins/com.mm.oidc/callback`).
### Proxy limitations

- JSON API clients that send `Accept: application/json` bypass the redirect by design; keep legacy automation pointed at the upstream `/api/v4` endpoints.
- WebSocket upgrades (`/api/v4/websocket`) are passed straight through—ensure your load balancer maintains the connection when chaining another proxy.
- If you run multiple Mattermost instances behind the proxy, configure sticky sessions or an external session store so `MMAUTHTOKEN` cookies stay valid across hosts.

---

## 3. Using the plugin with Mattermost Desktop App

The plugin supports authentication via the Mattermost Desktop application on Windows, macOS, and Linux. The desktop app opens the OAuth flow in your system's default browser, then redirects back to the app after successful authentication.

### Prerequisites

- Mattermost Desktop app v5.0+ installed and configured
- Plugin installed and configured as described in section 1
- System browser (Chrome, Firefox, Safari, Edge) available

### How it works

1. **Desktop app initiates login**: When you click "Sign in with SSO" or similar in the desktop app, it navigates to the plugin's login URL.
2. **Detection and external browser**: The plugin detects desktop/mobile clients (via the `isMobile=true` query parameter, which can be added by the client or by a proxy inspecting the User-Agent). For desktop clients, instead of a simple HTTP redirect, the plugin serves an HTML page that opens the OAuth provider (Keycloak) in your system's external browser using JavaScript `window.open()`.
3. **Browser authentication**: You complete the OIDC authentication flow in your external browser (which allows using saved passwords, password managers, security keys, etc.).
4. **Protocol handler redirect**: After successful authentication, the browser redirects to a `mattermost://` URL that the desktop app is registered to handle.
5. **Session handoff**: The desktop app receives the authentication tokens and establishes your session.

### Configuration

The plugin automatically detects desktop/mobile clients when the `isMobile=true` query parameter is present in the login URL.

**Option 1: Using the bundled proxy (Recommended)**  
If you front Mattermost with the bundled nginx proxy (or copy its rules), desktop/mobile requests are automatically detected via `User-Agent` header, and the proxy appends `isMobile=true` to `/plugins/com.mm.oidc/login` before forwarding to the plugin. This ensures OAuth opens in external browser without client-side changes.

**Option 2: Without proxy**  
The desktop app needs to add `?isMobile=true` to the login URL when initiating OAuth flows. Contact your desktop app administrator or configure your deployment to append this parameter for Mattermost desktop clients.

### Usage

1. **Launch the Mattermost Desktop app**
2. **Add or select your server**: Enter your Mattermost server URL (e.g., `https://chat.example.com`)
3. **Click to authenticate**: The app will detect that SSO is available and open your system browser
4. **Complete authentication**: Log in through your Identity Provider in the browser
5. **Return to app**: After successful authentication, you'll see a "Redirecting to Mattermost" page that automatically opens the desktop app
   - If automatic redirect doesn't work, click the "Click here to open Mattermost" button

### Troubleshooting

**Problem**: Browser redirects to Mattermost but desktop app doesn't open

**Solutions**:
- Ensure the desktop app is installed and running
- Check that the `mattermost://` protocol handler is registered (this happens automatically during installation)
- On Windows: Check Windows Settings → Apps → Default Apps → Choose default apps by protocol
- On macOS: The protocol handler should be registered automatically; try reinstalling the desktop app if it's not working
- On Linux: Check your desktop environment's protocol handler configuration

**Problem**: Desktop app opens but login doesn't complete

**Solutions**:
- Ensure you're running Desktop app v5.0 or newer
- Check that the server URL in the desktop app matches your plugin's `Redirect URL` configuration
- Review Mattermost server logs for any error messages
- Try clearing the desktop app's cache (File → Settings → Clear Cache and Restart)

**Problem**: "Missing authentication parameters" error

**Solutions**:
- This may indicate that the desktop app isn't properly passing the session tokens
- Try logging in via the web browser first to verify the plugin is working correctly
- Check that your Mattermost server's `SiteURL` is configured correctly in System Console

### Security notes

- **Browser-based authentication is more secure**: The desktop app opens OAuth in your system browser (not an embedded webview), which allows you to:
  - Verify you're on the correct login page
  - Use browser password managers
  - Use hardware security keys (FIDO2/WebAuthn)
  - Reuse existing authenticated sessions
- **Protocol handler security**: The `mattermost://` protocol ensures that authentication tokens are only passed to the legitimate Mattermost desktop application installed on your system
- **Short-lived tokens**: Authentication tokens passed via the protocol handler are short-lived and single-use

---
