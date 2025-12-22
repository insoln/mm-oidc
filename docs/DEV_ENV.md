# Local Dev Environment

This guide describes how to launch an ephemeral Mattermost + Keycloak stack that automatically mounts the current plugin bundle for integration and end-to-end testing.

## Overview

- Orchestrated via Docker Compose (`deploy/docker-compose.dev.yml`).
- Relies on Postgres backends for both Mattermost and Keycloak to mimic production-like persistence.
- Plugin bundles are sourced from `build/plugins/`. `scripts/dev-bootstrap.sh` now runs `make package` for you and ensures the resulting `mm-oidc.tar.gz` lives in that directory before pushing it into Mattermost.
- Keycloak bootstrap (realm + client) is handled by `scripts/dev-bootstrap.sh` via `kcadm`, keeping the plugin configuration in sync with the generated client secret.
- The stack defaults `KC_HOSTNAME` to `keycloak.127.0.0.1.nip.io`, a wildcard domain that resolves to `127.0.0.1` on your host while the compose network maps the same hostname back to the Keycloak container, so both browser traffic and in-cluster requests agree on a single issuer URL.
- Mattermost traffic now flows through the bundled NGINX proxy exposed at `http://mattermost-proxy.127.0.0.1.nip.io:8787`, matching the site URL configured in Mattermost and the redirect targets used by the plugin.
- Credentials, ports, and image tags are configured through `deploy/env/dev.env`.

## Prerequisites

- Docker 25+ with Compose V2 plugin.
- Ports `8787` (Mattermost via proxy), `8065` (direct Mattermost debug access), and `8080` (Keycloak) free on your host.
- Optional: GNU Make for the future `make dev-*` wrappers.

## First-Time Setup

1. Copy the env template and adjust values as needed:
   ```bash
   cp deploy/env/dev.env.example deploy/env/dev.env
   $EDITOR deploy/env/dev.env
   ```
2. Make sure you have the Go/Node/Yarn toolchain installed. The upcoming `scripts/dev-up.sh` run will call `make package` automatically and drop the archive into `build/plugins/`, so no manual copy is required unless you want to test a custom bundle.
3. Ensure the helper scripts are executable (already handled in git).

## Usage

### Start the stack

```bash
scripts/dev-up.sh
```

- Creates `build/plugins/` if missing.
- Seeds `deploy/env/dev.env` from the example on first run.
- Waits for services to become healthy before returning.
- Triggers `scripts/dev-bootstrap.sh`, which now builds the plugin (`make package`), ensures the seed `mm-admin` account exists, uploads the fresh archive under `/plugins/mm-oidc.tar.gz`, re-enables `com.mm.oidc` via `mmctl` on every run, provisions/assigns the Keycloak client role `system_admin`, and reconciles the required protocol mappers so ID tokens always expose the Mattermost-required claims.
- Logs into Keycloak using the credentials from `dev.env`, ensures the configured realm/client exist, syncs redirect/web origins, captures the client secret, and patches the plugin configuration so Mattermost always points at the freshly provisioned credentials.

### Follow logs

```bash
scripts/dev-logs.sh mattermost keycloak
```

Without arguments the script tails every service. Pass one or more service names to filter.

### Trigger the OIDC login flow manually

Once `scripts/dev-up.sh` installs the plugin you can exercise the end-to-end authentication flow without wiring any custom UI yet:

1. Open Mattermost in your browser at `http://mattermost-proxy.127.0.0.1.nip.io:8787` and log out if necessary.
2. Navigate to `http://mattermost-proxy.127.0.0.1.nip.io:8787/plugins/com.mm.oidc/` — the plugin now serves a minimal landing page with a **Start Login** button plus current issuer/redirect metadata.
3. Click **Start Login** to launch the Authorization Code + PKCE flow against the configured Keycloak realm. On success you will be redirected back to the Mattermost site URL with a valid `MMAUTHTOKEN` cookie, so the standard UI should show you as signed in as the provisioned user.

This landing page lives entirely within the plugin backend so it remains available even before we ship the React webapp bundle. It is safe to expose in dev/test environments but you should still rely on the regular Mattermost UX for production deployments once the webapp is in place.

If you are already signed in, open the plugin entry from the Mattermost product menu (the new Vite web bundle registers a root component). The in-product panel mirrors the `/health` data and exposes another **Start Login** button so admins can validate the flow without leaving the client.

### Stop and clean up

```bash
scripts/dev-down.sh
```

Removes containers and orphans but keeps named volumes so database state persists between runs. Use `docker compose ... down -v` manually if you want a full reset.

### Validate the proxy redirect rules

Run the bundled curl harness after the stack is up:

```bash
scripts/test-proxy.sh
```

To add the Playwright regression (requires the dev stack to stay up), use:

```bash
scripts/test-proxy-all.sh
```

Both commands read `deploy/env/dev.env` so they always target the active hostname/port.

## Working With Plugin Bundles

The bootstrap helper now handles packaging and upload automatically:

1. `scripts/dev-bootstrap.sh` invokes `make package`, which compiles the Go server, builds the React bundle, and emits `build/plugins/mm-oidc.tar.gz`.
2. The resulting archive is volume-mounted into the Mattermost container at `/plugins/mm-oidc.tar.gz` and pushed through `mmctl plugin add --force` so the running server always has the latest code.
3. Subsequent runs repeat the process, so simply calling `scripts/dev-up.sh` (or rerunning `scripts/dev-bootstrap.sh`) is enough to refresh the installed plugin.

If you ever need to build manually—for example, to inspect the tarball contents—`make package` is still available and will output to `build/plugins/mm-oidc.tar.gz`.

Mattermost looks for plugins inside `/plugins` (mounted from `build/plugins/`), so any `.tar.gz` placed there becomes available under **System Console → Plugin Management** for installation.

## Keycloak Bootstrap Automation

The bootstrap script now provisions the IdP alongside the plugin:

- `wait_for_keycloak` polls `http://localhost:${KC_HTTP_PORT}` until the admin realm (default `master`) exposes its discovery document.
- `kcadm config credentials` logs in with `KC_ADMIN/KC_ADMIN_PASSWORD` (realm can be overridden via `KC_ADMIN_REALM`).
- `ensure_oidc_realm` creates the target realm if it is not `master` and does not already exist.
- `ensure_oidc_client` creates or updates the confidential client defined by `OIDC_CLIENT_ID`. Redirect URIs are derived from `MM_SITE_URL + OIDC_REDIRECT_PATH`, and the issuer URL defaults to `http://${KC_HOSTNAME}:${KC_HTTP_PORT}/realms/${OIDC_REALM}` unless you override `OIDC_ISSUER_URL` (set to `http://keycloak.127.0.0.1.nip.io:8080/realms/master` by default).
- `ensure_keycloak_admin_email` sets `KC_ADMIN_EMAIL` on the built-in admin account (and marks it verified) so Keycloak always returns an `email` claim—required for automatic user provisioning in Mattermost.
- `ensure_oidc_client_mappers` keeps the standard profile + client-role protocol mappers (`preferred_username`, `given_name`, `family_name`, `full_name`, `email`, and `resource_access.<client>.roles`) present so the plugin receives consistent claims without manual Keycloak tweaks.
- `ensure_client_admin_role` (plus `ensure_admin_has_client_role`) creates the Keycloak client role `system_admin`—override via `OIDC_CLIENT_ADMIN_ROLE`—and grants it to the bootstrap `KC_ADMIN` user so the plugin can translate that claim into the Mattermost `system_admin` role.
- `ensure_admin_user` provisions the seed `mm-admin` user using the credentials from `dev.env`. The plugin now auto-converts existing password users to OIDC during login, so the account seamlessly switches to SSO the first time you authenticate via Keycloak.
- The retrieved client secret is written into Mattermost via `mmctl config set`, along with issuer, scopes, redirect URL, and the `OIDC_ALLOW_INSECURE` flag. The plugin is restarted so the new settings take effect immediately.

You can still open the Keycloak admin console (`http://localhost:8080/`) to inspect the generated realm/client or to tweak attributes manually. Re-running `scripts/dev-bootstrap.sh` will reconcile the settings back to the values derived from `dev.env`.

## Test Data

Default credentials (override in `dev.env`):

- Mattermost admin: `mm-admin / Password123!`
- Keycloak admin console: `admin / Keycloak123!`

The bootstrap automation creates (or updates) the `OIDC_REALM` and the `OIDC_CLIENT_ID`, so no manual client provisioning is required. Use the Keycloak UI if you need to verify values or experiment with additional settings; the next bootstrap run will restore the defaults from `dev.env`.

## CI Reuse

The same compose file can run inside GitHub Actions or any CI agent by invoking `docker compose --env-file deploy/env/dev.env.example -f deploy/docker-compose.dev.yml up --exit-code-from tests`. Future work will add a dedicated `make integration-test` target that spins up the stack, executes the Go/React integration suites, and tears it down automatically.
