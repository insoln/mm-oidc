#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
E2E_DIR="${ROOT_DIR}/e2e"
ENV_FILE="${ROOT_DIR}/deploy/env/dev.env"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
else
  echo -e "${YELLOW}[e2e-test] ${ENV_FILE} not found; falling back to default Playwright env vars.${NC}"
fi

echo -e "${GREEN}[e2e-test] Running Playwright E2E tests${NC}"

# Check if Node.js is available
if ! command -v node >/dev/null 2>&1; then
  echo -e "${RED}[e2e-test] Node.js is required but not installed.${NC}" >&2
  exit 1
fi

# Navigate to e2e directory
cd "${E2E_DIR}"

# Check if dependencies are installed
if [[ ! -d "node_modules" ]]; then
  echo -e "${YELLOW}[e2e-test] Installing Playwright dependencies...${NC}"
  corepack enable
  corepack yarn install --inline-builds
  
  echo -e "${YELLOW}[e2e-test] Installing Playwright browsers...${NC}"
  corepack yarn playwright install chromium
fi

# Check if dev stack should be started
if [[ "${SKIP_DEV_STACK:-false}" == "true" ]]; then
  echo -e "${YELLOW}[e2e-test] Skipping dev stack startup (SKIP_DEV_STACK=true)${NC}"
else
  echo -e "${YELLOW}[e2e-test] Ensuring dev stack is running...${NC}"
  "${ROOT_DIR}/scripts/dev-up.sh"
fi

# Run tests
echo -e "${GREEN}[e2e-test] Running Playwright tests...${NC}"
if [[ "${CI:-false}" == "true" ]]; then
  # In CI, rely on Playwright config for retries and reporting
  corepack yarn test
else
  # Locally, pass any additional arguments
  corepack yarn test "$@"
fi

EXIT_CODE=$?

if [[ $EXIT_CODE -eq 0 ]]; then
  echo -e "${GREEN}[e2e-test] ✓ All tests passed${NC}"
else
  echo -e "${RED}[e2e-test] ✗ Tests failed with exit code ${EXIT_CODE}${NC}"
  echo -e "${YELLOW}[e2e-test] View test report: yarn test:report${NC}"
fi

exit $EXIT_CODE
