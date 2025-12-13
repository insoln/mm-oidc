# Mattermost OIDC Plugin

Mattermost plugin that enables single sign-on with arbitrary OpenID Connect providers (Keycloak by default). The project tracks the latest Mattermost Server (v9.x+) and Keycloak (v25.x+) releases and follows strict security and coding best practices.

## Installation & Configuration

The plugin is distributed as a standard Mattermost archive. Download it from [GitHub Releases](https://github.com/insoln/mm-oidc/releases) or build it with `make package` (see Packaging below). Local builds output `build/plugins/mm-oidc.tar.gz`.

### 1. Install the plugin in Mattermost

1. Sign in to Mattermost as a system administrator.
2. Navigate to **System Console → Plugin Management → Plugin Upload** and enable plugin uploads if prompted.
3. Upload `mm-oidc.tar.gz` (from `build/plugins/` or the GitHub release) and click **Enable** for `com.mm.oidc`.
4. Open `https://<mattermost-host>/plugins/com.mm.oidc/` to verify the landing page renders and shows your configured issuer/redirect metadata.

### 2. Configure Keycloak (step-by-step)

These instructions assume Keycloak 25.x with the new admin console.

1. **Sign in** to the Keycloak Admin Console (`https://<keycloak-host>/admin`) using the bootstrap admin credentials.
2. **Create or select a realm**:
   - Click the realm selector (top-left) → **Create realm**.
   - Enter a realm name such as `mattermost` and click **Create**. Skip this if you already use a dedicated realm.
3. **Create the Mattermost client**:
   - In the left sidebar choose **Clients** → **Create client**.
   - Set **Client type** to *OpenID Connect*, **Client ID** to something memorable (e.g., `mattermost`), and click **Next**.
   - On the *Capability config* step:
     - Enable **Client authentication**.
     - Enable **Standard flow** (Authorization Code) and disable **Implicit flow**, **Direct access grants**, and **Service accounts**.
     - Click **Next**.
   - On *Login settings*:
     - **Valid redirect URIs**: `https://<mattermost-host>/plugins/com.mm.oidc/callback`.
     - **Web origins**: `https://<mattermost-host>` (add additional origins if you expose Mattermost on multiple hostnames).
     - Leave front-channel logout blank unless you plan to wire it later, then click **Save**.
4. **Capture the client secret**:
   - After saving, open the new client → **Credentials** tab → copy the **Client secret**. You will paste this into Mattermost configuration.
5. **Add protocol mappers** (Clients → your client → **Client scopes** → **Add mapper → By configuration**):
   - `preferred_username`: *User Property* mapper with property `username`, token claim name `preferred_username`, include in ID/UserInfo/Access tokens.
   - `given_name`: property `firstName`, claim `given_name`.
   - `family_name`: property `lastName`, claim `family_name`.
   - `full_name`: *Full name* mapper.
   - `email`: property `email`, claim `email`.
   - *(Optional)* Client role mapper: type **Client roles**, select your client, set token claim name `resource_access.<client_id>.roles`, enable multi-valued output. This allows the plugin to read Keycloak roles for admin promotion.
6. **Define an admin role (optional)**:
   - Still inside the client, open **Roles** → **Add role**.
   - Name it `system_admin` (or similar) and click **Save**.
   - Assign the role to privileged users: **Users** → select user → **Role mapping** → **Assign role** → choose the client + role.
7. **Verify email settings** (recommended): ensure user accounts have verified emails so Mattermost can auto-provision them. On the user profile page, set **Email verified** to *ON* if necessary.

At this point Keycloak exposes the issuer `https://<keycloak-host>/realms/<realm>`, a confidential client with proper scopes, and a client secret ready for the Mattermost plugin.

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

In **System Console → Plugins → Mattermost OIDC**, fill in:

- `Issuer URL`: `https://<keycloak-host>/realms/<realm>` (HTTPS strongly recommended in production).
- `Allow insecure issuer`: leave disabled unless you are on localhost with self-signed certs.
- `Client ID` / `Client Secret`: values from the Keycloak client you created.
- `Redirect URL`: `https://<mattermost-host>/plugins/com.mm.oidc/callback`.
- `Scopes`: typically `openid profile email`; include `roles` if you added the client-role mapper for admin promotion.
- Optional enforcement knobs such as domain allowlists or role-to-admin mapping (see future configuration UI).

Click **Save**, then use the **Start Login** button on the plugin landing page to complete a test round-trip. Successful authentication should provision the user automatically (including system-admin promotion if the Keycloak role is present).

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
  - `make package` emits `build/plugins/mm-oidc.tar.gz` (manifest + server binary + `webapp/dist/main.js`).
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
tar -czvf build/plugins/mm-oidc.tar.gz plugin.json server/dist/plugin-linux-amd64 webapp/dist/main.js
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
