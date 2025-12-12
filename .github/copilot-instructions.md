# Copilot Instructions

## Project Snapshot
- Build a Mattermost plugin that adds OIDC (Keycloak-first) SSO; latest Mattermost v9+ and Keycloak v25+ are the baseline (see `README.md`).
- Two code paths: Go backend in `server/` for Mattermost RPC/HTTP handlers and React/TypeScript web bundle in `webapp/` that injects UI into Mattermost and the System Console.
- Supporting assets live under `docs/` (architecture, dev env), `deploy/` (docker-compose, future Helm bits), `scripts/` (automation), and `build/` (packaged plugin artifacts).

## Architectural Expectations
- Server responsibilities (documented in `docs/ARCHITECTURE.md`): Authorization Code + PKCE flow, claim mapping, provisioning, config validation, KV-store session tracking, Zap logging, Prometheus metrics, OpenTelemetry traces.
- Webapp responsibilities: login CTA + admin-console configuration UI with client-side validation; bundle with Vite, typed with TS, styled via PostCSS modules.
- Security posture is strict: HTTPS issuers only, never log secrets, use nonce/state per request, encrypt refresh tokens before storing them.

## Development Workflow
- Toolchain versions: Go 1.22+, Node 20 LTS, Yarn 4 (Berry), Docker 25+, Make, jq.
- Primary commands (to be wired into the upcoming `Makefile`): `make server-lint/test`, `make webapp-lint/test`, `make package` (drops `build/<version>/mm-oidc.tar.gz`), `make dev-up` (alias for the script below).
- Local integration stack lives in `deploy/docker-compose.dev.yml`; use `scripts/dev-up.sh`, `scripts/dev-down.sh`, `scripts/dev-logs.sh` (documented in `docs/DEV_ENV.md`). Plugin archives placed in `build/plugins/` auto-mount into the Mattermost container.
- CI mirrors the same compose file for smoke/integration tests; plan future jobs under `.github/workflows/` to run lint → unit → Cypress/testcontainers suites.

## Conventions & Pitfalls
- Keep repo ASCII-only unless an existing file already uses Unicode; add only purposeful comments (see repo instructions).
- Secrets/config are always provided through env vars or Mattermost plugin KV store—never commit credentials. Redact sensitive fields in logs.
- Tests follow the described pyramid: Go unit tests per package, Jest/RTL for React, Cypress or Go contract tests that spin Keycloak/Mattermost via docker-compose or testcontainers.
- Package layout is stable; avoid ad-hoc folders. Place new docs in `docs/` (e.g., `docs/runbooks/`, `docs/threat-model/`) and update `README.md` when workflows change.
- Release artifacts must end up in `build/` with semantic versioned folders and signed `.tar.gz` bundles.

## When In Doubt
- Cross-check architecture decisions with `docs/ARCHITECTURE.md` and environment setup with `docs/DEV_ENV.md`.
- Prefer adding/adjusting automation via `scripts/` or future `Makefile` targets rather than ad-hoc shell commands baked into docs or CI.
- If implementing new flows, ensure observability hooks (structured logs, metrics, traces) are in place so the docker-compose stack surfaces useful diagnostics.
