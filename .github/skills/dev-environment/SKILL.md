---
name: dev-environment
description: Automates setup and management of the local development stack (Mattermost + Keycloak + Postgres + Nginx proxy) for the mm-oidc plugin. Use this skill when setting up a new development environment, starting/stopping the dev stack, viewing logs, or troubleshooting local development issues.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: development
  tags: [docker, dev-stack, local-development, automation]
---

# Dev Environment Skill

## Summary
This skill provides automated workflows for managing the local development environment for the Mattermost OIDC plugin. It leverages Docker Compose to orchestrate Mattermost, Keycloak, Postgres, and an Nginx proxy, with automatic plugin building, installation, and Keycloak bootstrap.

## When to Use This Skill
- Setting up a new development environment for the first time
- Starting or stopping the local dev stack
- Viewing logs from Mattermost, Keycloak, or other services
- Rebuilding and reinstalling the plugin during development
- Troubleshooting local environment issues
- Testing the complete OIDC flow locally

## Prerequisites
- Docker 25+ with Compose V2 plugin
- Go 1.22+ (for plugin building)
- Node.js 20.x with Corepack enabled (for webapp building)
- Ports 8787 (proxy), 8065 (Mattermost), and 8080 (Keycloak) available

## Key Operations

### Start the Development Stack
```bash
# Start everything (builds plugin, provisions Keycloak, configures Mattermost)
./scripts/dev-up.sh

# Or use the Makefile target
make dev-up
```

**What happens:**
1. Creates `build/plugins/` directory if missing
2. Seeds `deploy/env/dev.env` from example
3. Builds plugin using `make package`
4. Starts Docker containers
5. Waits for services to become healthy
6. Provisions Keycloak and configures Mattermost

### View Logs
```bash
# View all logs
./scripts/dev-logs.sh

# View specific services
./scripts/dev-logs.sh mattermost keycloak
```

### Stop the Stack
```bash
./scripts/dev-down.sh
```

### Access Services
- Mattermost (via proxy): http://mattermost-proxy.127.0.0.1.nip.io:8787
- Keycloak: http://keycloak.127.0.0.1.nip.io:8080

### Test OIDC Flow
1. Open http://mattermost-proxy.127.0.0.1.nip.io:8787
2. Navigate to `/plugins/com.mm.oidc/`
3. Click "Start Login"
4. Authenticate with Keycloak (admin / Keycloak123!)

## Configuration
Edit `deploy/env/dev.env` to customize credentials and settings.

## Troubleshooting

### Containers Won't Start
```bash
# Check ports
lsof -i :8787 :8065 :8080

# View status
docker compose -f deploy/docker-compose.dev.yml ps

# Check logs
./scripts/dev-logs.sh
```

### Plugin Not Loading
```bash
# Rebuild and upload
make package
docker compose -f deploy/docker-compose.dev.yml exec mattermost \
  mmctl plugin add --force /plugins/mm-oidc.tar.gz
```

### Reset Everything
```bash
./scripts/dev-down.sh
docker compose -f deploy/docker-compose.dev.yml down -v
./scripts/dev-up.sh
```

## Related Skills
- keycloak-setup
- plugin-build
- oidc-flow-test
- troubleshooting

## References
- [docs/DEV_ENV.md](../../../docs/DEV_ENV.md)
- [docs/DEVELOPER_GUIDE.md](../../../docs/DEVELOPER_GUIDE.md)
- [scripts/dev-up.sh](../../../scripts/dev-up.sh)
