#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.dev.yml"
ENV_FILE="${ROOT_DIR}/deploy/env/dev.env"

if ! command -v docker >/dev/null 2>&1; then
  echo "[dev-logs] Docker is required but not installed." >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[dev-logs] ${ENV_FILE} not found; did you run scripts/dev-up.sh?" >&2
  exit 1
fi

TARGETS=()
if [[ $# -gt 0 ]]; then
  TARGETS=("$@")
fi

docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" logs "${TARGETS[@]}"
