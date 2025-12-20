# Copilot Instructions

## Project Overview
This is a Mattermost plugin that enables single sign-on (SSO) with OpenID Connect providers, with Keycloak as the primary reference implementation. The plugin supports the latest Mattermost Server (v9.x+) and Keycloak (v25.x+) releases and follows strict security and coding best practices.

## Tech Stack
- **Backend**: Go 1.22+ (using Go modules, Mattermost plugin SDK)
- **Frontend**: React + TypeScript, bundled with Vite, styled with PostCSS modules
- **Testing**: Go unit tests, Vitest for React components, Cypress for E2E
- **Infrastructure**: Docker 25+, Docker Compose for local dev
- **Build Tools**: GNU Make, Yarn 4 (Berry via Corepack), jq

## Project Structure
```
.
├── server/                # Go backend plugin (Mattermost RPC entrypoints)
│   ├── plugin.go         # Main plugin lifecycle hooks
│   ├── config.go         # Configuration validation and management
│   ├── oidc.go           # OIDC flow implementation
│   ├── user.go           # User provisioning and management
│   └── *_test.go         # Unit tests for each package
├── webapp/               # React/TypeScript webapp bundle
│   ├── src/
│   │   ├── index.tsx     # Plugin entry point
│   │   ├── components/   # React components
│   │   └── utils/        # Utility functions
│   ├── tsconfig.json     # TypeScript configuration
│   └── vitest.config.ts  # Test configuration
├── docs/                 # Architecture, dev env, runbooks
│   ├── ARCHITECTURE.md   # Deep dive into components and flows
│   └── DEV_ENV.md        # Local development setup guide
├── deploy/               # IaC assets (docker-compose, future Helm charts)
├── scripts/              # Developer automation (dev-up.sh, dev-down.sh, dev-logs.sh)
├── build/                # Versioned plugin bundles and release notes
├── Makefile             # Build targets and automation
└── plugin.json          # Plugin manifest
```

## Coding Standards

### Go Backend
- Use Go modules and target Go 1.22+
- Follow standard Go naming conventions (camelCase for unexported, PascalCase for exported)
- Add godoc comments for all exported types, functions, and methods
- Structure comments: `// FunctionName does X` (not `// does X`)
- Use structured logging via Mattermost API: `p.API.LogError("message", "key", value)`
- Never log secrets or sensitive data; redact fields like `ClientSecret`
- Return meaningful errors with context: `fmt.Errorf("failed to X: %w", err)`
- Use thread-safe patterns (e.g., `sync.RWMutex` for shared state)
- Write unit tests for all packages (`*_test.go` files)
- Keep functions focused and single-purpose

### TypeScript/React Frontend
- Use TypeScript strict mode (`strict: true` in tsconfig.json)
- Use functional components with hooks (no class components)
- Explicitly type all props and state
- Use `import type` for type-only imports
- Follow React naming: PascalCase for components, camelCase for utilities
- Use CSS modules for styling (PostCSS)
- Write tests with Vitest and React Testing Library
- Export types/interfaces separately from implementation

### Testing Guidelines
- **Go**: Unit tests in `*_test.go` files, use table-driven tests where appropriate
- **React**: Component tests with Vitest + React Testing Library, test user interactions and state
- **Integration**: Use docker-compose stack for end-to-end validation
- Test coverage: Focus on critical paths (auth flows, config validation, user provisioning)
- Mock external dependencies (HTTP clients, Mattermost API calls)

## Security Requirements
- **HTTPS only**: Enforce HTTPS for issuer URLs (unless `AllowInsecureIssuer` is explicitly enabled for dev)
- **Never log secrets**: Redact `ClientSecret`, tokens, and sensitive claims in logs
- **State management**: Use nonce/state per auth request, stored in plugin KV store with short TTL
- **Token handling**: Keep ID/access tokens in-memory only; encrypt refresh tokens before persistence
- **Input validation**: Validate and sanitize all user inputs (URLs, redirect URIs, scopes)
- **PKCE required**: Use Authorization Code + PKCE flow for all OIDC interactions
- **Error messages**: Return sanitized error messages to clients; log detailed errors server-side

## Development Workflow

### Setup and Build
```bash
# Install dependencies
make webapp-install

# Build server
make server-build

# Build webapp
make webapp-build

# Create plugin bundle
make package
```

### Testing
```bash
# Run Go tests
make server-test

# Run webapp tests
make webapp-test

# Lint webapp (TypeScript check)
make webapp-lint
```

### Local Development Environment
```bash
# Start Mattermost + Keycloak stack
make dev-up   # or ./scripts/dev-up.sh

# View logs
make dev-logs SERVICES="mattermost keycloak"

# Stop stack
make dev-down
```

The dev stack automatically:
- Builds and installs the plugin into Mattermost
- Provisions Keycloak realm and client
- Configures plugin settings
- Maps plugin to `build/plugins/` directory

### Architectural Patterns
- **Server responsibilities**: Authorization Code + PKCE flow, claim mapping, user provisioning, config validation, KV-store session tracking, structured logging (Zap), metrics (Prometheus), tracing (OpenTelemetry)
- **Webapp responsibilities**: Login CTA, admin-console configuration UI, client-side validation
- **Configuration**: All secrets via env vars or Mattermost plugin KV store
- **Observability**: Structured logs with correlation IDs, Prometheus metrics for auth attempts/latency, OpenTelemetry traces

## Common Pitfalls
- Don't commit secrets or credentials to source code
- Don't create ad-hoc folders; use established structure
- Don't log sensitive data (tokens, secrets, PII)
- Don't use global state without proper locking
- Don't skip error handling or return generic errors
- Don't hardcode URLs or magic strings; use constants
- Avoid Unicode in code unless existing files use it
- Update documentation when changing workflows or APIs

## Important References
- **Architecture**: See `docs/ARCHITECTURE.md` for component details, security model, and flows
- **Dev Environment**: See `docs/DEV_ENV.md` for local setup and Keycloak bootstrap
- **Main README**: `README.md` for installation, configuration, and usage
- **Makefile**: Check `Makefile` for all available build targets
- **CI Workflows**: `.github/workflows/` for continuous integration pipelines

## Contribution Guidelines
- Use trunk-based development (feature branches merge to `main`)
- Every feature branch must include tests and documentation updates
- Run linters and tests before committing
- Write clear commit messages describing changes
- Update relevant documentation (README, ARCHITECTURE, DEV_ENV) when changing functionality
- Release artifacts go in `build/` with semantic versioning
- Use `make package` to create plugin bundles, never manually tar files

## When In Doubt
- Cross-check architectural decisions with `docs/ARCHITECTURE.md`
- Verify environment setup against `docs/DEV_ENV.md`
- Use `scripts/` for automation instead of ad-hoc shell commands
- Ensure observability hooks (logs, metrics, traces) are in place for new flows
- Ask for clarification rather than making assumptions about security or configuration
