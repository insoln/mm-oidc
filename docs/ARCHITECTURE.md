# Architecture Overview

This document captures the initial architecture for the Mattermost OIDC plugin. The solution targets the latest stable Mattermost Server (v9.x+) and Keycloak (v25.x+) releases and adheres to zero-trust and secure-by-default principles.

## Goals

- Provide frictionless OpenID Connect login to Mattermost using Authorization Code + PKCE.
- Support any compliant OIDC provider, with Keycloak serving as the primary reference implementation.
- Keep the plugin deployable in self-hosted and managed Mattermost environments.
- Offer strong observability, auditability, and resilience guarantees.

## Component Breakdown

### Server (`server/`)

- **Auth Handlers** – `/login` generates Authorization Code + PKCE redirects (state, nonce, code verifier, KV-backed session storage) while `/callback` exchanges the authorization code for tokens, verifies the `id_token` against the provider's JWKS, and provisions/updates Mattermost users (persisting OIDC subject → user mappings in the plugin KV store). `/logout` revokes the current Mattermost session, deletes the encrypted token bundle, clears cookies, and redirects the browser to the provider's `end_session_endpoint` when exposed.
- **Desktop Token Flow** – `/login/desktop` and `/login/desktop_token` endpoints support Mattermost desktop application authentication. When `desktop_token` query parameter is present, the callback generates server tokens and redirects to `mattermost://callback` URL scheme instead of setting session cookies. Desktop app exchanges these tokens for a session via the `/login/desktop_token` API endpoint.
- **Claim Mapper** – maps ID token / userinfo claims to Mattermost user fields, supports custom transformations and enforcement (e.g., domain allowlists, role mapping).
- **Provisioning Service** – creates or links Mattermost accounts, handles profile sync, and enforces plugin-specific policies (auto-provision vs. invite-only).
- **Config Manager** – validates admin-provided settings, caches OIDC discovery metadata, rotates secrets, and integrates with Mattermost's configuration store.
- **Storage** – leverages Mattermost plugin KV store for ephemeral session data (state, nonce) and encrypted refresh tokens.
- **Observability Hooks** – structured logging (Zap), metrics (Prometheus counters/histograms), and tracing integration via OpenTelemetry.

The server is written in Go, uses Go modules, and targets Go 1.22 or newer. The implementation includes configuration validation, health/login/callback HTTP endpoints, Authorization Code + PKCE flow, claim mapping, user provisioning, and session management via the plugin KV store.

### Webapp (`webapp/`)

- **Minimal Bundle** – The webapp provides a minimal plugin registration bundle (~281 bytes). 
- **Backend-Driven UI** – The plugin's landing page and login flow are served by the Go backend at `/plugins/com.mm.oidc/`.
- **Admin Console** – Configuration is managed through Mattermost's native plugin settings defined in `plugin.json`.
- **Future Extensibility** – The webapp structure supports future admin console customizations via `registerAdminConsoleCustomSetting`.

The webapp is built with React + TypeScript and Vite for bundling. It exports a minimal static bundle consumed by Mattermost during plugin load.

### Docs & Ops (`docs/`, `deploy/`, `scripts/`)

- `docs/` – Architecture overview, user guide, developer guide, environment setup (DEV_ENV.md), E2E testing guide, proxy guide, and Keycloak setup instructions.
- `deploy/` – Docker Compose development stack (`docker-compose.dev.yml`) with Mattermost, Keycloak, Postgres, and Nginx proxy; includes environment configuration templates.
- `scripts/` – Automation for dev stack management (`dev-up.sh`, `dev-down.sh`, `dev-logs.sh`), bootstrap scripts, and test harnesses (`e2e-test.sh`, `test-proxy.sh`).

## Configuration Model

Key settings exposed via the Mattermost System Console:

- OIDC issuer URL and discovery mode (auto vs. manual endpoints).
- Client ID / client secret (referenced via plugin secrets, never committed).
- Scopes, prompt behavior, PKCE requirement toggle (defaults to on).
- Claim mapping for username, email, display name, roles, and profile images.
- Domain allowlist, auto-provision flag, and logout redirect settings.
- Telemetry opt-in (anonymous usage metrics).

Validation logic ensures:

- HTTPS-only issuer endpoints.
- Secret redaction in logs.
- Regeneration of state/nonce per auth attempt.

## Security Considerations

- **PKCE + nonce/state** for all flows; state stored in the plugin KV store with short TTL.
- **Token handling** – ID/access tokens kept in-memory only; optional refresh tokens encrypted before persistence with per-installation keys.
- **Error handling** – sanitized messages to clients, detailed traces in server logs.
- **Logging** – structured JSON logs with correlation IDs for each login attempt.
- **Dependency hygiene** – Dependabot and `npm audit`/`govulncheck` integrated into CI.

## Desktop Application Support

The plugin supports authentication for Mattermost desktop applications (Windows, macOS, Linux) using a token-based flow compatible with the desktop app protocol:

### Desktop Authentication Flow

1. **Desktop-initiated login**: Desktop app opens `/plugins/com.mm.oidc/login?desktop_token=<client_token>` in the system browser
2. **OAuth flow**: User completes OIDC authentication with the provider (same as web flow)
3. **Token generation**: On callback, plugin detects `desktop_token` in session state and:
   - Generates a server token stored in KV store (3-minute TTL)
   - Redirects to `/plugins/com.mm.oidc/login/desktop` with both tokens
4. **Desktop redirect**: Browser auto-redirects to `mattermost://callback?client_token=...&server_token=...`
5. **Session creation**: Desktop app calls `/plugins/com.mm.oidc/login/desktop_token` API with server token to create session

### Key Implementation Details

- **Desktop token storage**: Server tokens stored in plugin KV store with 3-minute expiration
- **URL scheme handling**: Uses `mattermost://` protocol for production, `mattermost-dev://` for development builds
- **Security**: Tokens are single-use and automatically deleted after validation or expiration
- **Compatibility**: Tested with Mattermost Desktop v5.13+ and Server v10.11+

### Parameter Support

The `/login` endpoint accepts the following query parameters for desktop/mobile flows:
- `desktop_token` – Token provided by desktop app for session creation
- `redirect_to` – Optional redirect URL after successful authentication
- `mobile` – Boolean flag indicating mobile app flow (not yet fully implemented)

This implementation mirrors the desktop authentication flow from [Mattermost Server v10.11](https://github.com/mattermost/mattermost/tree/release-10.11), ensuring compatibility with current desktop app releases.

## Observability

- Server exports Prometheus metrics (e.g., `oidc_login_attempt_total`, `oidc_login_duration_seconds_bucket`).
- Distributed tracing hooks (OpenTelemetry) can emit spans when configured.
- Audit records written via Mattermost's audit interface for successful and failed logins.

## Testing Strategy

- **Unit tests** – Go tests (`*_test.go`) for handlers, claim mapping, config validation; Vitest for minimal webapp bundle.
- **End-to-end** – Playwright test suite validates complete OIDC flows against docker-compose stack (see [docs/E2E_TESTING.md](E2E_TESTING.md)).
- **Integration** – Docker Compose dev stack provides repeatable environment for manual and automated testing.
- **Security** – GitHub Advisory Database checks for dependencies, CodeQL scans for vulnerabilities.

## Release & Deployment

- CI builds versioned plugin bundles and uploads to `build/plugins/`.
- Docker Compose files in `deploy/` support local validation and development.
- Release notes summarize features, fixes, and security considerations; Semantic Versioning strictly enforced.
- Plugin can be deployed to any Mattermost v9.x+ instance via System Console upload or programmatic installation.
