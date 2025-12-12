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
- **Claim Mapper** – maps ID token / userinfo claims to Mattermost user fields, supports custom transformations and enforcement (e.g., domain allowlists, role mapping).
- **Provisioning Service** – creates or links Mattermost accounts, handles profile sync, and enforces plugin-specific policies (auto-provision vs. invite-only).
- **Config Manager** – validates admin-provided settings, caches OIDC discovery metadata, rotates secrets, and integrates with Mattermost's configuration store.
- **Storage** – leverages Mattermost plugin KV store for ephemeral session data (state, nonce) and encrypted refresh tokens.
- **Observability Hooks** – structured logging (Zap), metrics (Prometheus counters/histograms), and tracing integration via OpenTelemetry.

The server is written in Go, uses Go modules, and targets Go 1.22 or newer. The current codebase already includes the production skeleton: configuration validation, health/login/callback HTTP endpoints, and a reusable HTTP client. The actual Authorization Code + PKCE flow, claim mapping, and storage layers will be implemented on top of this foundation.

### Webapp (`webapp/`)

- **Login Entry Point** – React component injects a "Login with Keycloak" (or provider-specific) CTA into the Mattermost login UI. (The logout endpoint remains available for future wiring but currently has no first-party trigger.)
- **State Feedback** – surfaces error banners, success messages, and loading indicators derived from query parameters or Redux state.
- **Admin Console UI** – configuration form embedded in Mattermost's system console, validating input client-side and syncing with the server via the plugin API.

The webapp is built with React + TypeScript, PostCSS modules, and Vite for bundling. It exports a static bundle consumed by Mattermost during plugin load.

### Docs & Ops (`docs/`, `deploy/`, `scripts/`)

- `docs/` – architecture notes, runbooks, threat models, compliance checklists, plus environment guides (see `docs/DEV_ENV.md`).
- `deploy/` – Helm chart and Kustomize bases for packaging the plugin along with Mattermost & Keycloak dev stacks; contains `docker-compose.dev.yml` for local/CI smoke tests.
- `scripts/` – repeatable automation (bootstrap dev env, generate certificates, run integration suites).

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

## Observability

- Server exports Prometheus metrics (e.g., `oidc_login_attempt_total`, `oidc_login_duration_seconds_bucket`).
- Distributed tracing hooks (OpenTelemetry) can emit spans when configured.
- Audit records written via Mattermost's audit interface for successful and failed logins.

## Testing Strategy

- **Unit tests** – Go tests for handlers, claim mapping, config validation; Jest + React Testing Library for UI.
- **Contract tests** – use `ory/dockertest` or `testcontainers-go` to spin up ephemeral Keycloak + Mattermost instances verifying auth flows.
- **End-to-end** – Cypress suite running against docker-compose stack in CI.
- **Security tests** – ZAP baseline scans and dependency scanning as part of release workflow.

## Release & Deployment

- CI builds versioned plugin bundles, signs them, and uploads to `build/<version>`.
- `deploy/helm` packages facilitate installation into Kubernetes clusters; docker-compose files support local validation.
- Release notes summarize features, fixes, and security considerations; Semantic Versioning strictly enforced.

This document is the starting point—update it as implementation details solidify.
