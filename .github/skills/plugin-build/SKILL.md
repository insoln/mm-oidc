---
name: plugin-build
description: Builds the Go server backend, React webapp frontend, and packages the complete plugin bundle for the Mattermost OIDC plugin. Use this skill when building from source, preparing releases, or during incremental development.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: development
  tags: [build, golang, react, webpack, vite, packaging]
---

# Plugin Build Skill

## Summary
This skill guides the build process for the Mattermost OIDC plugin, including Go backend compilation, React webapp bundling with Vite, and creating the final plugin archive.

## When to Use This Skill
- Building plugin from source code
- Preparing plugin for installation or deployment
- Creating release artifacts
- Incremental development (rebuild after code changes)
- CI/CD pipeline integration
- Troubleshooting build issues

## Prerequisites
- Go 1.22+
- Node.js 20.x with Corepack enabled
- Yarn 4 (Berry) via Corepack
- Make utility

## Build Process Overview

The plugin consists of two parts:
1. **Server (Go)**: Backend plugin that implements OIDC flow
2. **Webapp (React + TypeScript)**: Frontend UI components

## Full Build Commands

### Complete Plugin Package
```bash
# Build everything and create plugin archive
make package

# Output: build/plugins/mm-oidc.tar.gz
```

This command:
1. Builds Go server binary for Linux AMD64
2. Builds React webapp with Vite
3. Creates plugin directory structure
4. Packages everything into tarball

### Incremental Builds

```bash
# Build server only
make server-build
# Output: server/dist/plugin-linux-amd64

# Build webapp only (installs dependencies first)
make webapp-build
# Output: webapp/dist/main.js

# Install webapp dependencies
make webapp-install
```

## Detailed Build Steps

### Server Build

```bash
cd server
GOOS=linux GOARCH=amd64 go build -o dist/plugin-linux-amd64 ./...
```

**What this does:**
- Compiles Go code to Linux AMD64 binary
- Targets production deployment (Linux servers)
- Outputs to `server/dist/plugin-linux-amd64`

**Build options:**
```bash
# Development build (faster, includes debug info)
go build -o dist/plugin-linux-amd64 ./...

# Production build (optimized)
go build -ldflags="-s -w" -o dist/plugin-linux-amd64 ./...

# Cross-compilation for other platforms
GOOS=darwin GOARCH=arm64 go build -o dist/plugin-darwin-arm64 ./...
```

### Webapp Build

```bash
cd webapp
corepack yarn install --inline-builds
corepack yarn build
```

**What this does:**
- Installs npm dependencies via Yarn 4
- Runs Vite build with TypeScript compilation
- Outputs bundled JavaScript to `webapp/dist/main.js`

**Build options:**
```bash
# Development build (faster, not minified)
yarn build:dev

# Production build (minified, optimized)
yarn build

# Watch mode (rebuild on changes)
yarn build --watch
```

### Package Assembly

```bash
make package
```

**Directory structure created:**
```
build/package/com.mm.oidc/
├── plugin.json
├── server/
│   └── dist/
│       └── plugin-linux-amd64
└── webapp/
    └── dist/
        └── main.js
```

**Archive created:**
```
build/plugins/mm-oidc.tar.gz
```

## Testing Builds

### Server Tests
```bash
# Run all Go tests
make server-test

# Or manually
cd server
go test ./...

# With coverage
go test -cover ./...

# With verbose output
go test -v ./...
```

### Webapp Tests
```bash
# Run all webapp tests
make webapp-test

# Or manually
cd webapp
yarn test

# Watch mode
yarn test:watch

# Coverage report
yarn test:coverage
```

### Linting
```bash
# Lint webapp (TypeScript check)
make webapp-lint

# Or manually
cd webapp
yarn lint
```

## Build Output Verification

```bash
# Check server binary
ls -lh server/dist/plugin-linux-amd64
file server/dist/plugin-linux-amd64

# Check webapp bundle
ls -lh webapp/dist/main.js

# Check plugin archive
ls -lh build/plugins/mm-oidc.tar.gz
tar -tzf build/plugins/mm-oidc.tar.gz
```

## Common Build Issues

### Go Build Fails

**Problem:** `go: no such file or directory`
```bash
# Solution: Ensure Go is installed
go version

# Install dependencies
cd server
go mod download
go mod tidy
```

**Problem:** `undefined: SomeFunc`
```bash
# Solution: Rebuild with fresh dependencies
go clean -cache
go build ./...
```

### Webapp Build Fails

**Problem:** `Cannot find module`
```bash
# Solution: Reinstall dependencies
cd webapp
rm -rf node_modules
corepack yarn install --inline-builds
```

**Problem:** TypeScript errors
```bash
# Solution: Check TypeScript configuration
yarn tsc --noEmit

# Or skip type checking (not recommended)
yarn build --no-type-check
```

**Problem:** Vite build hangs
```bash
# Solution: Increase Node memory
NODE_OPTIONS="--max-old-space-size=4096" yarn build
```

### Package Creation Fails

**Problem:** `No such file: server/dist/plugin-linux-amd64`
```bash
# Solution: Build server first
make server-build
make package
```

**Problem:** `No such file: webapp/dist/main.js`
```bash
# Solution: Build webapp first
make webapp-build
make package
```

## Clean Build

```bash
# Remove all build artifacts
make clean-plugin

# Manual cleanup
rm -f build/plugins/mm-oidc.tar.gz
rm -f server/dist/plugin-linux-amd64
rm -rf webapp/dist
```

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Build plugin
  run: |
    make webapp-install
    make package
    
- name: Upload artifact
  uses: actions/upload-artifact@v3
  with:
    name: plugin-bundle
    path: build/plugins/mm-oidc.tar.gz
```

### Docker Build
```dockerfile
FROM golang:1.22 AS server-build
WORKDIR /build
COPY server/ ./server/
RUN cd server && GOOS=linux GOARCH=amd64 go build -o dist/plugin-linux-amd64 ./...

FROM node:20 AS webapp-build
WORKDIR /build
COPY webapp/ ./webapp/
RUN cd webapp && corepack enable && yarn install && yarn build

FROM scratch AS package
COPY --from=server-build /build/server/dist /package/com.mm.oidc/server/dist
COPY --from=webapp-build /build/webapp/dist /package/com.mm.oidc/webapp/dist
COPY plugin.json /package/com.mm.oidc/
```

## Development Workflow

### Quick Iteration Cycle
```bash
# 1. Make code changes
vim server/plugin.go

# 2. Rebuild server
make server-build

# 3. Repackage
make package

# 4. Reinstall in dev stack
docker compose -f deploy/docker-compose.dev.yml exec mattermost \
  mmctl plugin add --force /plugins/mm-oidc.tar.gz
```

### Watch Mode (Webapp)
```bash
# Terminal 1: Watch webapp changes
cd webapp
yarn build --watch

# Terminal 2: Watch server changes (manually rebuild)
# Use entr or similar tool
ls server/*.go | entr -r make server-build
```

## Build Optimization

### Faster Builds
```bash
# Skip tests during build
make server-build webapp-build package

# Parallel builds (if supported)
make -j4 package
```

### Smaller Bundle Size
```bash
# Analyze webapp bundle
cd webapp
yarn build
yarn analyze

# Enable tree shaking
# Configure in vite.config.ts
```

## Release Build Checklist

- [ ] Update version in `plugin.json`
- [ ] Run all tests: `make server-test webapp-test`
- [ ] Run linter: `make webapp-lint`
- [ ] Clean build: `make clean-plugin && make package`
- [ ] Verify archive contents: `tar -tzf build/plugins/mm-oidc.tar.gz`
- [ ] Test installation in clean Mattermost instance
- [ ] Run E2E tests: `./scripts/e2e-test.sh`
- [ ] Generate release notes
- [ ] Tag release: `git tag v0.0.2`

## Related Skills
- dev-environment
- e2e-testing
- release-management
- troubleshooting

## References
- [Makefile](../../../Makefile)
- [docs/DEVELOPER_GUIDE.md](../../../docs/DEVELOPER_GUIDE.md)
- [server/](../../../server/)
- [webapp/](../../../webapp/)
