# Copilot Instructions

## Project Overview
This is a Mattermost plugin that enables single sign-on (SSO) with OpenID Connect providers, with Keycloak as the primary reference implementation. The plugin supports the latest Mattermost Server (v9.x+) and Keycloak (v25.x+) releases and follows strict security and coding best practices.

## Tech Stack
- **Backend**: Go 1.22+ (using Go modules, Mattermost plugin SDK)
- **Frontend**: React + TypeScript, bundled with Vite, styled with PostCSS modules
- **Testing**: Go unit tests, Vitest for React components, Playwright for E2E
- **Infrastructure**: Docker 25+, Docker Compose for local dev
- **Build Tools**: GNU Make, Yarn 4 (Berry via Corepack), jq

## Project Structure
```
.
├── server/                # Go backend plugin (Mattermost RPC entrypoints)
├── webapp/               # React/TypeScript webapp bundle
├── docs/                 # Architecture, dev env, runbooks
├── deploy/               # IaC assets (docker-compose, future Helm charts)
├── scripts/              # Developer automation (dev-up.sh, dev-down.sh, dev-logs.sh)
├── build/                # Versioned plugin bundles and release notes
├── .github/skills/       # Agent Skills for AI-assisted development
└── Makefile             # Build targets and automation
```

## Coding Standards

### Go Backend
- Use Go modules and target Go 1.22+
- Follow standard Go naming conventions (camelCase for unexported, PascalCase for exported)
- Add godoc comments for all exported types, functions, and methods
- Use structured logging via Mattermost API: `p.API.LogError("message", "key", value)`
- Never log secrets or sensitive data; redact fields like `ClientSecret`
- Return meaningful errors with context: `fmt.Errorf("failed to X: %w", err)`
- Use thread-safe patterns (e.g., `sync.RWMutex` for shared state)
- Write unit tests for all packages (`*_test.go` files)

### TypeScript/React Frontend
- Use TypeScript strict mode (`strict: true` in tsconfig.json)
- Use functional components with hooks (no class components)
- Explicitly type all props and state
- Use CSS modules for styling (PostCSS)
- Write tests with Vitest and React Testing Library

## Security Requirements
- **HTTPS only**: Enforce HTTPS for issuer URLs (unless `AllowInsecureIssuer` is explicitly enabled for dev)
- **Never log secrets**: Redact `ClientSecret`, tokens, and sensitive claims in logs
- **State management**: Use nonce/state per auth request, stored in plugin KV store with short TTL
- **Token handling**: Keep ID/access tokens in-memory only; encrypt refresh tokens before persistence
- **PKCE required**: Use Authorization Code + PKCE flow for all OIDC interactions
- **Error messages**: Return sanitized error messages to clients; log detailed errors server-side

## Agent Skills
For detailed workflows, refer to `.github/skills/`:
- **dev-environment** - Local development stack setup
- **keycloak-setup** - IdP configuration
- **plugin-build** - Build and packaging
- **plugin-install** - Installation and configuration
- **oidc-flow-test** - Authentication testing
- **proxy-setup** - Reverse proxy deployment
- **troubleshooting** - Diagnostics and issue resolution
- **user-provisioning** - User management
- **security-audit** - Security review
- **observability** - Logging, metrics, tracing
- **config-migration** - Configuration management
- **e2e-testing** - Playwright tests
- **health-check** - Component health validation
- **release-management** - Version and release management

## Quick Reference

### Build Commands
```bash
make package           # Build complete plugin bundle
make server-build      # Build Go backend only
make webapp-build      # Build React frontend only
```

### Development
```bash
make dev-up            # Start local dev stack
make dev-logs          # View service logs
make dev-down          # Stop dev stack
```

### Testing
```bash
make server-test       # Run Go tests
make webapp-test       # Run webapp tests
make e2e-test          # Run Playwright E2E tests
```

## Important References
- **Architecture**: `docs/ARCHITECTURE.md` - Component details, security model, flows
- **Dev Environment**: `docs/DEV_ENV.md` - Local setup and Keycloak bootstrap
- **User Guide**: `docs/USER_GUIDE.md` - Installation and configuration
- **Developer Guide**: `docs/DEVELOPER_GUIDE.md` - Build, test, and release workflows
- **Agent Skills**: `.github/skills/README.md` - Complete skill catalog

## When In Doubt
- Check relevant Agent Skill in `.github/skills/` for detailed workflows
- Cross-check architectural decisions with `docs/ARCHITECTURE.md`
- Verify environment setup against `docs/DEV_ENV.md`
- Use `scripts/` for automation instead of ad-hoc shell commands
