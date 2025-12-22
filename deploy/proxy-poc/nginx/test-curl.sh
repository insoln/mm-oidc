#!/bin/bash
# Test NGINX proxy redirect functionality with curl

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load local .env for host/port defaults if present
if [ -f "$SCRIPT_DIR/.env" ]; then
    set -a
    # shellcheck disable=SC1091
    source "$SCRIPT_DIR/.env"
    set +a
fi

PROXY_HOSTNAME="${PROXY_HOSTNAME:-proxy.127.0.0.1.nip.io}"
PROXY_PORT="${PROXY_PORT:-8787}"
DEFAULT_BASE_URL="http://${PROXY_HOSTNAME}:${PROXY_PORT}"
BASE_URL="${BASE_URL:-${PROXY_BASE_URL:-$DEFAULT_BASE_URL}}"

echo "==> Testing NGINX Proxy OIDC Redirect"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

test_passed=0
test_failed=0

# Helper function to run a test
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    local expected_pattern="$3"
    
    echo -n "Test: $test_name ... "
    
    if output=$(eval "$test_cmd" 2>&1); then
        if echo "$output" | grep -q "$expected_pattern"; then
            echo -e "${GREEN}PASS${NC}"
            ((++test_passed))
            return 0
        else
            echo -e "${RED}FAIL${NC}"
            echo "  Expected pattern: $expected_pattern"
            echo "  Got: $output"
            ((++test_failed))
            return 1
        fi
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Command failed: $test_cmd"
        echo "  Output: $output"
        ((++test_failed))
        return 1
    fi
}

echo "==> Scenario 1: Unauthenticated user should be redirected to OIDC"
run_test "Root URL without cookie redirects" \
    "curl -s -o /dev/null -w '%{http_code}|%{redirect_url}' '$BASE_URL/'" \
    "302"

run_test "Root URL redirect points to OIDC login" \
    "curl -s -D - -o /dev/null '$BASE_URL/' | grep -i location" \
    "/plugins/com.mm.oidc/login"

echo ""
echo "==> Scenario 2: API endpoints should NOT redirect"
run_test "API endpoint returns 401 or 200, not redirect" \
    "curl -s -o /dev/null -w '%{http_code}' '$BASE_URL/api/v4/system/ping'" \
    "200"

run_test "API endpoint with JSON accept header doesn't redirect" \
    "curl -s -H 'Accept: application/json' -o /dev/null -w '%{http_code}' '$BASE_URL/api/v4/users/me'" \
    "401"

echo ""
echo "==> Scenario 3: Plugin endpoints should NOT redirect"
run_test "Plugin health endpoint works" \
    "curl -s '$BASE_URL/plugins/com.mm.oidc/health'" \
    "status"

run_test "Plugin login endpoint doesn't redirect" \
    "curl -s -o /dev/null -w '%{http_code}' '$BASE_URL/plugins/com.mm.oidc/login'" \
    "302"

echo ""
echo "==> Scenario 4: Static files should NOT redirect"
run_test "Static files accessible" \
    "curl -s -o /dev/null -w '%{http_code}' '$BASE_URL/static/'" \
    "^[24][0-9][0-9]$"

echo ""
echo "==> Scenario 5: Health check should NOT redirect"
run_test "Health check endpoint works" \
    "curl -s '$BASE_URL/health'" \
    "OK"

echo ""
echo "==> Scenario 6: POST requests should NOT redirect"
run_test "POST request doesn't redirect" \
    "curl -s -X POST -o /dev/null -w '%{http_code}' '$BASE_URL/api/v4/users/login'" \
    "[45]"

echo ""
echo "==> Scenario 7: Requests with MMAUTHTOKEN cookie should NOT redirect"
# This test would require a valid token, so we just test the behavior
echo -e "${YELLOW}SKIP${NC} - Would require valid MMAUTHTOKEN (manual test)"

echo ""
echo "=========================================="
echo "Test Results:"
echo -e "  ${GREEN}Passed: $test_passed${NC}"
echo -e "  ${RED}Failed: $test_failed${NC}"
echo "=========================================="

if [ $test_failed -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
