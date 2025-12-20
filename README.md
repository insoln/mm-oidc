# Mattermost OIDC Plugin

This plugin gives Mattermost a production-grade OIDC bridge (Authorization Code + PKCE) with Keycloak-first defaults, hardened state/nonce handling, encrypted refresh tokens, and observability hooks. It targets Mattermost Server v9.x+ and Keycloak v25.x+ while remaining compatible with any standards-compliant provider.

## Installation & Configuration

Install the packaged archive in Mattermost, then point it at a confidential client inside your IdP. You can download releases from [GitHub Releases](https://github.com/insoln/mm-oidc/releases) or run `make package`, which produces `build/plugins/mm-oidc.tar.gz`.

### 1. Install the plugin in Mattermost

1. Sign in to Mattermost as a system administrator.
2. Navigate to **System Console → Plugin Management → Plugin Upload** and enable uploads if prompted.
3. Upload `mm-oidc.tar.gz` (either from `build/plugins/` or a release asset) and click **Enable** for `com.mm.oidc`.
4. Visit `https://<mattermost-host>/plugins/com.mm.oidc/` to confirm the landing page renders; it should list the issuer and redirect metadata once configured.

### 2. Configure Keycloak (step-by-step)

These steps target Keycloak 25.x and its current admin console.

1. **Sign in** to `https://<keycloak-host>/admin` with your bootstrap admin.
2. **Create or select a realm** dedicated to Mattermost (realm selector → **Create realm** → name such as `mattermost`).
3. **Create the Mattermost client**:
  - **Clients → Create client** → choose *OpenID Connect* and set a memorable **Client ID**.
  - On **Capability config**, enable **Client authentication** and **Standard flow**; disable **Implicit flow**, **Direct access grants**, and **Service accounts**.
  - On **Login settings**, set **Valid redirect URIs** to `https://<mattermost-host>/plugins/com.mm.oidc/callback` and **Web origins** to `https://<mattermost-host>` (add alternates as needed). Leave front-channel logout empty unless you plan to wire it later. Click **Save**.
4. **Capture the client secret** from the **Credentials** tab. You will paste this into Mattermost.
5. **Add protocol mappers** (Clients → your client → **Client scopes** → **Add mapper → By configuration**):
  - `preferred_username`: User Property `username`, claim `preferred_username`, include in ID/UserInfo/Access tokens.
  - `given_name`: property `firstName`, claim `given_name`.
  - `family_name`: property `lastName`, claim `family_name`.
  - `full_name`: Full Name mapper.
  - `email`: property `email`, claim `email`.
  - *(Optional)* Client role mapper: type **Client roles**, claim `resource_access.<client_id>.roles`, multi-valued output (enables admin promotion).
6. **Define an admin role (optional)**: Clients → your client → **Roles** → **Add role** (e.g., `system_admin`). Assign it via **Users → Role mapping** for anyone who should become a Mattermost System Admin.
7. **Confirm email verification** so Mattermost can trust addresses (Users → profile → set **Email verified** to *ON* when needed).

After this, Keycloak exposes the issuer `https://<keycloak-host>/realms/<realm>` plus a confidential client with mappers and a ready-to-use secret.

#### Classic Admin Console (Keycloak ≤17)

If you still use the legacy console (`https://<host>/auth/admin/master/console/`):

1. **Realm** – open the realm dropdown (top-left) → click **Add Realm** → supply a name (e.g., `mattermost`) → **Create**.
2. **Client** – go to **Clients** → **Create** → enter `Client ID` (e.g., `mattermost`) and choose **OpenID Connect** → **Save**.
3. **Settings tab**:
  - **Access Type** → *Confidential*.
  - Enable **Standard Flow Enabled**; disable **Implicit Flow** and **Direct Access Grants**.
  - **Valid Redirect URIs** → `https://<mattermost-host>/plugins/com.mm.oidc/callback`.
  - **Web Origins** → `https://<mattermost-host>` (or `+` to add more origins).
  - Click **Save**.
4. **Credentials tab** – set **Client Authenticator** to *Client Id and Secret*, then copy the generated **Secret**.
5. **Mappers tab** – click **Create** repeatedly and add:
  - *User Property* mapper for `preferred_username` (User Property = `username`, Token Claim Name = `preferred_username`, include in ID & Access tokens).
  - Similar mappers for `given_name` (`firstName`), `family_name` (`lastName`), `email` (`email`).
  - **Full Name** mapper (built-in) for `full_name`.
  - *(Optional)* **User Client Role** mapper with Token Claim Name `resource_access.<client_id>.roles` and Multivalued = *On*.
6. **Roles tab** – create `system_admin` (optional) → **Users** → select user → **Role Mappings** → assign the new client role to any administrators who should become Mattermost System Admins.
7. **Users** – ensure `Email Verified` is checked for each account so the plugin can trust the claim.

Legacy and modern console settings are equivalent; only the navigation differs.

### 3. Configure the plugin settings

Inside **System Console → Plugins → Mattermost OIDC** provide:

- `Issuer URL`: `https://<keycloak-host>/realms/<realm>` (always HTTPS outside disposable labs).
- `Allow insecure issuer`: disable unless you are developing against `http://` endpoints.
- `Client ID` and `Client Secret`: values from the Keycloak client.
- `Redirect URL`: `https://<mattermost-host>/plugins/com.mm.oidc/callback`.
- `Scopes`: usually `openid profile email`; append `roles` if you configured the client-role mapper.
- Optional enforcement knobs (domain restrictions, role mapping) as they land in future UI updates.

Click **Save** and press **Start Login** on the plugin landing page to validate the round-trip. A successful run provisions the user and, if the Keycloak role is present, promotes them to Mattermost System Admin.

> 💡 Tip: `scripts/dev-up.sh` + `scripts/dev-bootstrap.sh` perform every step above automatically for the Docker-based dev stack. Refer to `docs/DEV_ENV.md` if you prefer automation over manual configuration.

### Limitations: Login/Logout Intercepts

Mattermost plugins cannot override the core `/login` or `/logout` pages; only the official Enterprise SAML/OIDC features hook those routes. This plugin exposes its own landing page under `/plugins/com.mm.oidc/` and issues redirects from there, so users must click **Start Login** (or an equivalent CTA injected by the webapp) instead of using the stock forms.

**Workarounds**

- **Ingress rewrite**: Configure your reverse proxy/ingress to redirect `/login` (and optionally `/logout`) to `/plugins/com.mm.oidc/login`. This keeps the default entry points but requires extra care:
  - Ensure health checks and API/login automation bypass the rewrite (e.g., only rewrite browser traffic, not `/api/v4/users/login`).
  - Preserve CSRF cookies and query strings when you redirect so Mattermost’s own forms still work for local/system accounts.
- **Custom UI link**: Hide the stock login link in your Mattermost theme and surface a “Sign in with Keycloak” button that points to `/plugins/com.mm.oidc/login`.

**Pitfalls**

- Redirect loops occur if the ingress blindly rewrites Mattermost’s callback requests; scope the rule to GET requests without `code/state` params.
- Automated clients (CLI integrations, legacy bots) that rely on username/password auth will fail if `/api/v4/users/login` is blocked—leave API endpoints untouched.
- Session/logout flows still rely on Mattermost cookies; if you force `/logout` through the plugin, make sure the plugin route ultimately sends the user back to `/logout` so server-side session cleanup runs.

## Project Layout

```
.
├── build/                 # Versioned plugin bundles and release notes
├── deploy/                # IaC assets (Helm charts, Kubernetes manifests)
├── docs/                  # Architecture, runbooks, threat models
├── scripts/               # Developer automation (lint, package, e2e)
├── server/                # Go backend plugin (Mattermost RPC entrypoints)
├── webapp/                # React/TypeScript webapp bundle
└── .github/workflows/     # Continuous integration pipelines
```

Additional files (created as implementation progresses):

- `Makefile` – canonical entrypoint for linting, testing, packaging, and releasing.
- `go.mod` / `package.json` – language toolchains pinned to secure versions.
- `docs/ARCHITECTURE.md` – deep dive into components, flows, and security (see first draft inside `docs/`).
## Development Workflow

### Team practices

- **Branching**: trunk-based development. Every feature branch ships with tests + documentation updates before merging to `main`.
- **Quality gates**: `golangci-lint` for Go, `eslint`/`stylelint`/`tsc --noEmit` for the webapp, `hadolint` for container artifacts.
- **Security**: Dependabot/Snyk (or equivalent) stay enabled, secrets live only in environment variables or the encrypted Mattermost plugin KV store.
- **Testing**: prioritize unit tests, add contract/integration tests for the OIDC flow, and keep Cypress suites for full-stack validation.
- **Releases**: tag every release, publish signed `.tar.gz` bundles in `build/`, and aggregate changelog fragments per PR.

### Tooling & local environment

- **Prerequisites**: Go 1.22+, Node.js 20 LTS, Yarn 4 (Berry via Corepack), Docker 25+, GNU Make, `jq`.
- **Environment scripts**: `scripts/dev-up.sh` brings up Mattermost + Keycloak via docker-compose and runs `scripts/dev-bootstrap.sh`; `scripts/dev-down.sh` tears it down; `scripts/dev-logs.sh [service ...]` tails containers.
- **Make targets**:
  - `make server-test` / `make server-build` for Go tests and linux/amd64 builds.
  - `make webapp-build`, `make webapp-test`, `make webapp-lint` for the React bundle.
  - `make package` emits `build/plugins/mm-oidc.tar.gz` (contents staged under `com.mm.oidc/` with the manifest, server binary, and entire `webapp/dist/`).
  - `make dev-up`, `make dev-down`, `make dev-logs` wrap the scripts above.

### Server internals

`server/` exposes the `/health`, `/login`, `/callback`, and `/logout` handlers. `/login` launches Authorization Code + PKCE (state/nonce persisted in the plugin KV store), `/callback` exchanges the code, validates the `id_token`, provisions or links Mattermost users (including role synchronization via `ensureSystemRoles`), and writes encrypted refresh tokens to storage. A shared HTTP client with sane timeouts plus thread-safe router/metadata caches keeps everything resilient.

### Packaging

```bash
make package
```

- Produces a linux/amd64 bundle compatible with docker-compose and official releases. Override `GOOS/GOARCH` inside the Makefile for experimental builds.
- `plugin.json` references `server/dist/plugin-linux-amd64` and `webapp/dist/main.js`; run `make webapp-build` beforehand so the assets exist.
- Keep the resulting archive under `build/plugins/` so `scripts/dev-up.sh` can mount it automatically.

Manual fallback:

```bash
cd server
GOOS=linux GOARCH=amd64 go build -o dist/plugin-linux-amd64 ./...
cd ../webapp
corepack yarn install
corepack yarn build
cd ..
mkdir -p build/package/com.mm.oidc/server/dist build/package/com.mm.oidc/webapp/dist
cp plugin.json build/package/com.mm.oidc/
cp server/dist/plugin-linux-amd64 build/package/com.mm.oidc/server/dist/
cp -R webapp/dist/. build/package/com.mm.oidc/webapp/dist/
tar -czvf build/plugins/mm-oidc.tar.gz -C build/package com.mm.oidc
```

### Webapp bundle

`webapp/` hosts the Vite + React + TypeScript bundle that injects the login CTA, mirrors `/health`, and links to `/plugins/com.mm.oidc/login`.

Key commands:

```bash
cd webapp
corepack yarn install        # once per clone
corepack yarn dev            # Vite dev server with HMR
corepack yarn build          # -> webapp/dist/main.js
corepack yarn test           # Vitest (jsdom)
```

`tsc --noEmit` runs as part of `yarn build`, so typing issues fail fast even without a dev server.

## Continuous Integration

- Workflow: `.github/workflows/ci.yml`
- Triggers: push, pull_request, or manual `workflow_dispatch`
- Jobs:
  - `Go Server Tests` → Go 1.22 toolchain + `make server-test`.
  - `Webapp Tests` → Node 20 + Corepack, caches Yarn installs, runs `yarn test`.
  - `Dev Stack Bootstrap` → spins up the docker-compose stack via `scripts/dev-up.sh`, validates container health, and always runs `scripts/dev-down.sh` for cleanup.

All jobs must pass before merging. Extend the workflow with linting, integration, or packaging gates as the project grows.

## Documentation

- Architecture deep dive: `docs/ARCHITECTURE.md`
- Dev/test environment guide: `docs/DEV_ENV.md`
- Runbooks (planned): `docs/runbooks/`
- Threat model & security reviews (planned): `docs/threat-model/`

Every contribution should update relevant docs and tests to keep the repo production-ready.
