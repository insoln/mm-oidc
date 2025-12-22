#!/bin/bash
# Start the NGINX proxy POC stack

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

cd "$SCRIPT_DIR"

echo "==> Starting NGINX Proxy POC for Mattermost OIDC"

# Copy .env.example to .env if it doesn't exist
if [ ! -f .env ]; then
    echo "==> Creating .env from .env.example"
    cp .env.example .env
fi

# Build the plugin package first
echo "==> Building plugin package..."
cd "$PROJECT_ROOT"
make package || {
    echo "ERROR: Failed to build plugin package"
    exit 1
}

# Return to proxy POC directory
cd "$SCRIPT_DIR"

# Start services
echo "==> Starting Docker Compose stack..."
docker compose up -d

echo "==> Waiting for services to be healthy..."
sleep 5

# Wait for Mattermost
echo "==> Waiting for Mattermost..."
for i in {1..30}; do
    if docker compose exec -T mattermost wget -q --spider http://localhost:8065/api/v4/system/ping 2>/dev/null; then
        echo "==> Mattermost is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Mattermost failed to start"
        exit 1
    fi
    sleep 2
done

# Wait for Keycloak
echo "==> Waiting for Keycloak..."
for i in {1..30}; do
    if docker compose exec -T keycloak curl -sf http://localhost:8080/health/ready > /dev/null 2>&1; then
        echo "==> Keycloak is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Keycloak failed to start"
        exit 1
    fi
    sleep 2
done

# Bootstrap the environment
echo "==> Bootstrapping Mattermost and Keycloak..."
"$PROJECT_ROOT/scripts/dev-bootstrap.sh" "$SCRIPT_DIR"

echo ""
echo "==> NGINX Proxy POC is ready!"
echo ""
echo "Access points:"
echo "  - Mattermost (via NGINX):  http://localhost"
echo "  - Keycloak (direct):       http://localhost:8080"
echo ""
echo "Test the proxy redirect:"
echo "  curl -v http://localhost/ 2>&1 | grep Location"
echo ""
echo "View logs:"
echo "  docker compose logs -f nginx"
echo "  docker compose logs -f mattermost"
echo ""
