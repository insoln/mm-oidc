# Mattermost OIDC Plugin – User Guide

This guide walks system administrators through installing, configuring, and validating the OIDC plugin across three common scenarios:

1. Adding the plugin to an already running Mattermost cluster.
2. Exercising the plugin in the standalone Docker stack that ships with this repo.
3. Fronting Mattermost with the provided proxy container (or an equivalent Ingress/Nginx deployment) so `/login` automatically reroutes users into the plugin flow.

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
   - Alternate: run `make package` and copy `build/plugins/mm-oidc.tar.gz` to the host that can access System Console.
2. **Provision (or reuse) a confidential client in your IdP**
  - Follow the dedicated Keycloak instructions in [docs/KEYCLOAK_SETUP.md](docs/KEYCLOAK_SETUP.md) or replicate them for your IdP of choice.
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
   - Link the plugin landing page (`/plugins/com.mm.oidc/`) from your login experience or add a reverse-proxy rule (see section 3) so the plugin drives every interactive login.

### Verification

Run the full Playwright regression locally to confirm every documented step succeeds:

```bash
./scripts/e2e-test.sh
```

The script compiles the plugin if needed, starts the dev stack, and executes `tests/oidc-flow.spec.ts`, mirroring the manual steps above.

### Known limitations in vanilla Mattermost

- The plugin **cannot override** the stock `/login` and `/logout` routes; only `/plugins/com.mm.oidc/` and `/plugins/com.mm.oidc/login` launch the flow.
- Mobile apps and legacy password clients must continue using the built-in authentication methods until you add a proxy rule or custom UI entry point.
- Server metrics/logging already redact secrets, but Mattermost still displays raw IdP URLs inside the System Console; secure that interface appropriately.

---

## 2. Use the plugin inside the standalone dev stack

The repository ships with a fully automated Docker Compose stack that bootstraps Mattermost, Keycloak, Postgres backends, and the proxy container. This is ideal for local validation, demos, and CI.

### Start the stack

```bash
./scripts/dev-up.sh
```

What the script does:

1. Copies `deploy/env/dev.env.example` to `deploy/env/dev.env` on first run and exports every variable.
2. Runs `make package` to build `build/plugins/mm-oidc.tar.gz`.
3. Launches [deploy/docker-compose.dev.yml](deploy/docker-compose.dev.yml) and waits for healthy containers.
4. Executes [scripts/dev-bootstrap.sh](scripts/dev-bootstrap.sh) to:
   - Provision the Keycloak realm/client, required mappers, and the `system_admin` client role.
   - Create the Mattermost user `mm-admin` and grant the System Admin permission when the Keycloak role is present.
   - Upload and enable the freshly built plugin, then sync all plugin settings.

### Exercise the login flow

1. Open `http://mattermost-proxy.127.0.0.1.nip.io:8787/` in a browser (the proxy preserves the configured site URL).
2. Sign out of Mattermost if you are already logged in.
3. Navigate to `/plugins/com.mm.oidc/` or use the “Start Login” button inside the left-hand plugin panel.
4. Authenticate with the seeded Keycloak admin (`admin / Keycloak123!`).
5. After the redirect, confirm the Mattermost UI shows you as `mm-admin` and that the `MMAUTHTOKEN` cookie exists in your browser.

### Shut everything down

```bash
./scripts/dev-down.sh
```

Volumes stay intact so you can resume later. Use `docker compose -f deploy/docker-compose.dev.yml down -v` if you need a clean reset.

### Dev-stack limitations

- The stack intentionally exposes HTTP endpoints on `127.0.0.1.nip.io` and should **never** be published to the internet.
- SMTP/SMS MFA integrations are not wired; only username/password authentication is available in Keycloak.
- The proxy rewrites `/login` only for browser GET requests—API requests or CLI logins still hit the upstream Mattermost port (`8065`).

---

## 3. Deploy the proxy container (Docker Compose or Kubernetes)

Redirecting `/login` to `/plugins/com.mm.oidc/login` requires a front proxy that can inspect cookies and selectively rewrite requests. The repository ships with a ready-to-use Nginx container that you can copy into your own stack.

### 3.1 Docker Compose

1. **Import the service definition**
   - Reuse the service block from [deploy/docker-compose.dev.yml](deploy/docker-compose.dev.yml#L55-L140) and adjust the `ports`, `MM_SITE_URL`, and hostname values to match your environment.
   - Mount the maintained config [deploy/mattermost-proxy/nginx.conf](deploy/mattermost-proxy/nginx.conf) via a bind mount or ConfigMap.
2. **Wire the network**
   - Place the proxy and Mattermost containers on the same Docker network.
   - Update Mattermost’s `SiteURL` to `http(s)://<proxy-host>` so cookies and the plugin’s redirect metadata stay consistent.
3. **Expose the proxy**
   - Publish the proxy’s port(s) to your load balancer or ingress; the upstream Mattermost port should remain internal.
4. **Validate**
   - Run `./scripts/test-proxy.sh` to execute the curl smoke tests described in [docs/PROXY_IMPLEMENTATION_SUMMARY.md](docs/PROXY_IMPLEMENTATION_SUMMARY.md#тесты).
   - Follow up with `./scripts/test-proxy-all.sh` to execute the browser suite `tests/proxy-redirect.spec.ts`.

### 3.2 Kubernetes (example)

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
5. **CI validation** – bake the Playwright regression into your pipeline by running `./scripts/test-proxy-all.sh` against a staging namespace (requires port-forwarding or an exposed endpoint).

### Proxy limitations

- JSON API clients that send `Accept: application/json` bypass the redirect by design; keep legacy automation pointed at the upstream `/api/v4` endpoints.
- WebSocket upgrades (`/api/v4/websocket`) are passed straight through—ensure your load balancer maintains the connection when chaining another proxy.
- If you run multiple Mattermost instances behind the proxy, configure sticky sessions or an external session store so `MMAUTHTOKEN` cookies stay valid across hosts.

---

## Surfacing the instructions to end users

- The main README links directly to this guide for administrators.
- CLI/automation steps (packaging, testing, architecture deep dives) continue to live under `docs/`, and the README now points developers to those references instead of duplicating them here.

Use this document as the canonical reference when rolling out or troubleshooting the plugin; every update must stay in sync with the automated Playwright suites so the documented steps remain accurate.
