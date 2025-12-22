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

# Load environment variables for host-side checks
set -a
source .env
set +a

PROXY_HOSTNAME="${PROXY_HOSTNAME:-proxy.127.0.0.1.nip.io}"
PROXY_PORT="${PROXY_PORT:-8787}"
KC_HOSTNAME="${KC_HOSTNAME:-keycloak.127.0.0.1.nip.io}"
KC_HTTP_PORT="${KC_HTTP_PORT:-8080}"

PROXY_BASE_URL="http://${PROXY_HOSTNAME}:${PROXY_PORT}"
KEYCLOAK_BASE_URL="http://${KC_HOSTNAME}:${KC_HTTP_PORT}"

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

# Wait for Mattermost via NGINX proxy from the host
MM_PING_URL="${PROXY_BASE_URL}/api/v4/system/ping"
echo "==> Waiting for Mattermost (via ${MM_PING_URL})..."
for i in {1..30}; do
    if curl -sf -H "Accept: application/json" "$MM_PING_URL" > /dev/null 2>&1; then
        echo "==> Mattermost is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Mattermost failed to start"
        exit 1
    fi
    sleep 2
done

# Wait for Keycloak via host-exposed port
KEYCLOAK_CHECK_URL="${KEYCLOAK_BASE_URL}/"
echo "==> Waiting for Keycloak (via ${KEYCLOAK_CHECK_URL})..."
for i in {1..30}; do
    if curl -sfI "$KEYCLOAK_CHECK_URL" > /dev/null 2>&1; then
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
echo "  - Mattermost (via NGINX):  ${PROXY_BASE_URL}"
echo "  - Keycloak (direct):       ${KEYCLOAK_BASE_URL}"
echo ""
echo "Test the proxy redirect:"
echo "  curl -v ${PROXY_BASE_URL}/ 2>&1 | grep Location"
echo ""
echo "View logs:"
echo "  docker compose logs -f nginx"
echo "  docker compose logs -f mattermost"
echo ""
