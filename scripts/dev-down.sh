#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.dev.yml"
ENV_FILE="${ROOT_DIR}/deploy/env/dev.env"

if ! command -v docker >/dev/null 2>&1; then
  echo "[dev-down] Docker is required but not installed." >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[dev-down] ${ENV_FILE} not found; nothing to shut down." >&2
  exit 0
fi

echo "[dev-down] Stopping stack and removing containers..."
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down --remove-orphans "$@"
