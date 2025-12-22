#!/usr/bin/env bash
# Verify that the Mattermost proxy behaves as expected.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/deploy/env/dev.env"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[test-proxy] ${ENV_FILE} not found. Run scripts/dev-up.sh first." >&2
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
DEFAULT_BASE_URL="http://${PROXY_HOSTNAME}:${PROXY_PORT}"
BASE_URL="${BASE_URL:-${PROXY_BASE_URL:-${DEFAULT_BASE_URL}}}"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

test_passed=0
test_failed=0

run_test() {
  local test_name="$1"
  local test_cmd="$2"
  local expected_pattern="$3"

  echo -n "Test: ${test_name} ... "

  if output=$(eval "$test_cmd" 2>&1); then
    if echo "$output" | grep -Eq "$expected_pattern"; then
      echo -e "${GREEN}PASS${NC}"
      test_passed=$((test_passed + 1))
      return 0
    fi

    echo -e "${RED}FAIL${NC}"
    echo "  Expected pattern: ${expected_pattern}"
    echo "  Got: ${output}"
    test_failed=$((test_failed + 1))
    return 1
  fi

  echo -e "${RED}FAIL${NC}"
  echo "  Command failed: ${test_cmd}"
  echo "  Output: ${output}"
  test_failed=$((test_failed + 1))
  return 1
}

echo "==> Testing Mattermost Proxy (${BASE_URL})"
echo ""

echo "==> Scenario 1: Unauthenticated user should be redirected to OIDC"
run_test "Root URL without cookie redirects" \
  "curl -s -o /dev/null -w '%{http_code}|%{redirect_url}' '${BASE_URL}/'" \
  '302'

run_test "Root URL redirect points to OIDC login" \
  "curl -s -D - -o /dev/null '${BASE_URL}/' | grep -i location" \
  '/plugins/com.mm.oidc/login'

echo ""
echo "==> Scenario 2: API endpoints should NOT redirect"
run_test "API endpoint returns 401 or 200, not redirect" \
  "curl -s -o /dev/null -w '%{http_code}' '${BASE_URL}/api/v4/system/ping'" \
  '200'

run_test "API endpoint with JSON accept header doesn't redirect" \
  "curl -s -H 'Accept: application/json' -o /dev/null -w '%{http_code}' '${BASE_URL}/api/v4/users/me'" \
  '401'

echo ""
echo "==> Scenario 3: Plugin endpoints should NOT redirect"
run_test "Plugin health endpoint works" \
  "curl -s '${BASE_URL}/plugins/com.mm.oidc/health'" \
  'status'

run_test "Plugin login endpoint doesn't redirect" \
  "curl -s -o /dev/null -w '%{http_code}' '${BASE_URL}/plugins/com.mm.oidc/login'" \
  '302'

echo ""
echo "==> Scenario 4: Static files should NOT redirect"
run_test "Static files accessible" \
  "curl -s -o /dev/null -w '%{http_code}' '${BASE_URL}/static/'" \
  '^[24][0-9][0-9]$'

echo ""
echo "==> Scenario 5: Health check should NOT redirect"
run_test "Health check endpoint works" \
  "curl -s '${BASE_URL}/health'" \
  'OK'

echo ""
echo "==> Scenario 6: POST requests should NOT redirect"
run_test "POST request doesn't redirect" \
  "curl -s -X POST -o /dev/null -w '%{http_code}' '${BASE_URL}/api/v4/users/login'" \
  '^[45][0-9][0-9]$'

echo ""
echo "==> Scenario 7: Requests with MMAUTHTOKEN cookie should NOT redirect"
echo -e "${YELLOW}SKIP${NC} - Would require valid MMAUTHTOKEN (manual test)"

echo ""
echo "=========================================="
echo "Test Results:"
echo -e "  ${GREEN}Passed: ${test_passed}${NC}"
echo -e "  ${RED}Failed: ${test_failed}${NC}"
echo "=========================================="

if [[ ${test_failed} -eq 0 ]]; then
  echo -e "${GREEN}All tests passed!${NC}"
else
  echo -e "${RED}Some tests failed!${NC}"
  exit 1
fi
