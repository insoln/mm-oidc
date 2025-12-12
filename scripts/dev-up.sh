#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.dev.yml"
ENV_DIR="${ROOT_DIR}/deploy/env"
ENV_FILE="${ENV_DIR}/dev.env"
ENV_TEMPLATE="${ENV_DIR}/dev.env.example"
PLUGINS_DIR="${ROOT_DIR}/build/plugins"

if ! command -v docker >/dev/null 2>&1; then
  echo "[dev-up] Docker is required but not installed." >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "[dev-up] Docker Compose V2 plugin is required." >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  if [[ ! -f "${ENV_TEMPLATE}" ]]; then
    echo "[dev-up] Missing env template at ${ENV_TEMPLATE}" >&2
    exit 1
  fi
  cp "${ENV_TEMPLATE}" "${ENV_FILE}"
  echo "[dev-up] Seeded ${ENV_FILE}. Review credentials before continuing." >&2
fi

mkdir -p "${PLUGINS_DIR}"

echo "[dev-up] Bringing up Mattermost + Keycloak stack..."
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d --wait "$@"

"${ROOT_DIR}/scripts/dev-bootstrap.sh"

echo "[dev-up] Services are starting. Access Mattermost at http://localhost:8065 and Keycloak at http://localhost:8080"
