#!/usr/bin/env bash
# Run curl + Playwright proxy verification against the dev stack.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.dev.yml"
ENV_FILE="${ROOT_DIR}/deploy/env/dev.env"
PROXY_TEST_SCRIPT="${ROOT_DIR}/scripts/test-proxy.sh"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[test-proxy-all] ${ENV_FILE} not found. Run scripts/dev-up.sh first." >&2
  exit 1
fi

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

PROXY_HOSTNAME="${PROXY_HOSTNAME:-mattermost-proxy.127.0.0.1.nip.io}"
PROXY_PORT="${PROXY_PORT:-8787}"
export PROXY_BASE_URL="${PROXY_BASE_URL:-http://${PROXY_HOSTNAME}:${PROXY_PORT}}"

if ! docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" ps | grep -q mattermost-proxy; then
  echo "[test-proxy-all] Dev stack is not running. Start it via scripts/dev-up.sh first." >&2
  exit 1
fi

if ! docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" ps | grep -q "Up"; then
  echo "[test-proxy-all] One or more services are down. Check docker compose ps." >&2
  exit 1
fi

echo "==> 1. Running curl smoke tests"
"${PROXY_TEST_SCRIPT}"

echo ""
echo "==> 2. Running Playwright proxy suite"
export SKIP_DEV_STACK=1
export KC_ADMIN="${KC_ADMIN:-admin}"
export KC_ADMIN_PASSWORD="${KC_ADMIN_PASSWORD:-Keycloak123!}"

pushd "${ROOT_DIR}/e2e" >/dev/null
if [[ ! -d node_modules ]]; then
  corepack yarn install --inline-builds
fi
corepack yarn playwright test tests/proxy-redirect.spec.ts --reporter=list
popd >/dev/null

echo ""
echo "All proxy tests completed successfully."
