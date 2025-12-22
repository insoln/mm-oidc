#!/bin/bash
# Stop the NGINX proxy POC stack

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$SCRIPT_DIR"

echo "==> Stopping NGINX Proxy POC..."
docker compose down

echo "==> NGINX Proxy POC stopped"
echo ""
echo "To remove volumes (clean slate), run:"
echo "  docker compose down -v"
echo ""
