#!/bin/bash
# Validate NGINX configuration syntax

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Validating NGINX configuration syntax"

# Create a temporary config that replaces upstream with localhost for testing
TEMP_CONF=$(mktemp)
trap "rm -f $TEMP_CONF" EXIT

# Replace "mattermost:8065" with "localhost:8065" for validation
sed 's/server mattermost:8065;/server localhost:8065;/' "${SCRIPT_DIR}/nginx.conf" > "$TEMP_CONF"

# Test NGINX config syntax using Docker
docker run --rm \
  -v "${TEMP_CONF}:/etc/nginx/nginx.conf:ro" \
  nginx:1.25-alpine \
  nginx -t

echo ""
echo "==> NGINX configuration syntax is valid!"
echo ""
echo "Note: This validates syntax only. Full functionality requires"
echo "      the upstream 'mattermost' service to be available in the"
echo "      Docker Compose network."
