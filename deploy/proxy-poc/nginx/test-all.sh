#!/bin/bash
# Run all tests for the NGINX proxy POC

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

if [ -f "$SCRIPT_DIR/.env" ]; then
    set -a
    # shellcheck disable=SC1091
    source "$SCRIPT_DIR/.env"
    set +a
fi

PROXY_HOSTNAME="${PROXY_HOSTNAME:-proxy.127.0.0.1.nip.io}"
PROXY_PORT="${PROXY_PORT:-8787}"
KC_HOSTNAME="${KC_HOSTNAME:-keycloak.127.0.0.1.nip.io}"
KC_HTTP_PORT="${KC_HTTP_PORT:-8080}"
export PROXY_BASE_URL="${PROXY_BASE_URL:-http://${PROXY_HOSTNAME}:${PROXY_PORT}}"

echo "==> Running NGINX Proxy POC Tests"
echo ""

# Check if services are running
if ! docker compose ps | grep -q "Up"; then
    echo "ERROR: Services are not running. Please run ./start.sh first."
    exit 1
fi

echo "==> 1. Running curl tests..."
echo ""
"$SCRIPT_DIR/test-curl.sh"

echo ""
echo "==> 2. Running Playwright E2E tests..."
echo ""

# Set environment variables for E2E tests
export KC_ADMIN="${KC_ADMIN:-admin}"
export KC_ADMIN_PASSWORD="${KC_ADMIN_PASSWORD:-Keycloak123!}"
export SKIP_DEV_STACK=1

cd "$PROJECT_ROOT/e2e"

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "Installing E2E test dependencies..."
    yarn install
fi

# Run only proxy redirect tests
yarn playwright test proxy-redirect.spec.ts --reporter=list

echo ""
echo "==> All tests completed!"
echo ""
