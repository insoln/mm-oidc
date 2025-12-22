#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Allow override of compose directory (for proxy POC)
COMPOSE_DIR="${1:-${ROOT_DIR}/deploy}"
COMPOSE_FILE="${COMPOSE_DIR}/docker-compose.yml"
# Fallback to dev.yml if docker-compose.yml doesn't exist
if [[ ! -f "${COMPOSE_FILE}" ]]; then
  COMPOSE_FILE="${COMPOSE_DIR}/docker-compose.dev.yml"
fi

# Use .env from compose directory if it exists, otherwise use deploy/env
if [[ -f "${COMPOSE_DIR}/.env" ]]; then
  ENV_FILE="${COMPOSE_DIR}/.env"
else
  ENV_DIR="${ROOT_DIR}/deploy/env"
  ENV_FILE="${ENV_DIR}/dev.env"
fi
PLUGINS_DIR="${ROOT_DIR}/build/plugins"
PLUGIN_ID="com.mm.oidc"
PLUGIN_ARCHIVE_NAME="mm-oidc.tar.gz"
PLUGIN_HOST_PATH="${PLUGINS_DIR}/${PLUGIN_ARCHIVE_NAME}"
PLUGIN_CONTAINER_PATH="/plugins/${PLUGIN_ARCHIVE_NAME}"
OIDC_CLIENT_SECRET=""
OIDC_EFFECTIVE_ISSUER=""
OIDC_REDIRECT_URL=""
OIDC_CLIENT_INTERNAL_ID=""
OIDC_CLIENT_ADMIN_ROLE=""
KC_ADMIN_USER_ID=""

if ! command -v python3 >/dev/null 2>&1; then
  echo "[dev-bootstrap] python3 is required for JSON processing." >&2
  exit 1
fi

log() {
  echo "[dev-bootstrap] $*"
}

run_mmctl() {
  docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T mattermost /mattermost/bin/mmctl --local "$@"
}

kcadm() {
  docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T keycloak /opt/keycloak/bin/kcadm.sh "$@"
}

keycloak_login() {
  local keycloak_base="http://${KC_HOSTNAME}:${KC_HTTP_PORT}"
  kcadm config credentials \
    --server "${keycloak_base}" \
    --realm "${KC_ADMIN_REALM}" \
    --user "${KC_ADMIN}" \
    --password "${KC_ADMIN_PASSWORD}" \
    >/dev/null
}

wait_for_keycloak() {
  local keycloak_base="http://${KC_HOSTNAME}:${KC_HTTP_PORT}"
  log "Waiting for Keycloak to become ready on ${keycloak_base}..."
  local attempts=0
  until curl -sf "${keycloak_base}/realms/${KC_ADMIN_REALM}/.well-known/openid-configuration" >/dev/null; do
    attempts=$((attempts + 1))
    if [ "${attempts}" -gt 30 ]; then
      echo "Keycloak did not become ready in time" >&2
      exit 1
    fi
    sleep 2
  done
}

wait_for_mattermost() {
  local attempts=30
  local delay=2
  for ((i = 1; i <= attempts; i++)); do
    if run_mmctl system status >/dev/null 2>&1; then
      log "Mattermost is responding."
      return 0
    fi
    log "Waiting for Mattermost API (${i}/${attempts})..."
    sleep "${delay}"
  done
  log "Timed out waiting for Mattermost to become ready." >&2
  return 1
}

ensure_admin_user() {
  log "Ensuring admin user ${MM_ADMIN_USERNAME} exists..."
  if run_mmctl user search "${MM_ADMIN_USERNAME}" >/dev/null 2>&1; then
    log "Admin user ${MM_ADMIN_USERNAME} already present."
    return 0
  fi

  log "Creating admin user ${MM_ADMIN_USERNAME}."
  run_mmctl user create \
    --email "${MM_ADMIN_EMAIL}" \
    --username "${MM_ADMIN_USERNAME}" \
    --password "${MM_ADMIN_PASSWORD}" \
    --system-admin >/dev/null
  log "Admin user ${MM_ADMIN_USERNAME} created."
}

build_plugin() {
  log "Building plugin package via make package."
  if ! make -C "${ROOT_DIR}" package >/dev/null; then
    log "Plugin packaging failed." >&2
    exit 1
  fi

  if [[ ! -f "${PLUGIN_HOST_PATH}" ]]; then
    log "Expected plugin archive ${PLUGIN_HOST_PATH} missing after build." >&2
    exit 1
  fi

  log "Plugin bundle ready at ${PLUGIN_HOST_PATH}."
}

ensure_plugin_installed() {
  if [[ ! -f "${PLUGIN_HOST_PATH}" ]]; then
    log "Skipping plugin install; bundle ${PLUGIN_HOST_PATH} not found."
    return 0
  fi

  if ! run_mmctl plugin list | grep -q "${PLUGIN_ID}"; then
    log "Uploading plugin ${PLUGIN_ID} from ${PLUGIN_CONTAINER_PATH} (force replace)."
    run_mmctl plugin add --force "${PLUGIN_CONTAINER_PATH}" >/dev/null
  fi

  log "Enabling plugin ${PLUGIN_ID}."
  run_mmctl plugin enable "${PLUGIN_ID}" >/dev/null || run_mmctl plugin enable "${PLUGIN_ID}" >/dev/null
}

ensure_oidc_realm() {
  if [[ "${OIDC_REALM}" == "master" ]]; then
    return 0
  fi

  if kcadm get realms/"${OIDC_REALM}" >/dev/null 2>&1; then
    log "Keycloak realm ${OIDC_REALM} already exists."
    return 0
  fi

  log "Creating Keycloak realm ${OIDC_REALM}."
  kcadm create realms -s realm="${OIDC_REALM}" -s enabled=true >/dev/null
}

ensure_keycloak_admin_email() {
  if [[ -z "${KC_ADMIN_EMAIL}" ]]; then
    log "KC_ADMIN_EMAIL is empty; skipping admin email sync."
    return 0
  fi

  local admin_query
  admin_query=$(kcadm get users -r "${KC_ADMIN_REALM}" -q username="${KC_ADMIN}" 2>/dev/null || true)
  local admin_id
  admin_id=$(printf '%s' "${admin_query}" | resolve_keycloak_first_id)

  if [[ -z "${admin_id}" ]]; then
    log "Unable to locate admin user ${KC_ADMIN} in realm ${KC_ADMIN_REALM}." >&2
    exit 1
  fi

  KC_ADMIN_USER_ID="${admin_id}"

  local admin_detail
  admin_detail=$(kcadm get users/"${admin_id}" -r "${KC_ADMIN_REALM}")
  mapfile -t admin_fields < <(
    printf '%s' "${admin_detail}" | python3 -c 'import json,sys
data=json.load(sys.stdin)
print(data.get("email",""))
print(str(data.get("emailVerified", False)).lower())'
  )
  local current_email="${admin_fields[0]:-}"
  local email_verified="${admin_fields[1]:-false}"

  if [[ "${current_email}" == "${KC_ADMIN_EMAIL}" && "${email_verified}" == "true" ]]; then
    log "Keycloak admin ${KC_ADMIN} already has email ${KC_ADMIN_EMAIL}."
    return 0
  fi

  log "Updating Keycloak admin ${KC_ADMIN} email to ${KC_ADMIN_EMAIL}."
  kcadm update users/"${admin_id}" -r "${KC_ADMIN_REALM}" \
    -s "email=${KC_ADMIN_EMAIL}" \
    -s emailVerified=true >/dev/null
}

resolve_keycloak_first_id() {
  python3 -c '
import json, sys
payload = sys.stdin.read().strip()
if not payload:
  sys.exit(0)
try:
  data = json.loads(payload)
except json.JSONDecodeError:
  sys.exit(0)
if isinstance(data, list) and data:
  print(data[0].get("id", ""))
'
}

ensure_admin_user_id_cached() {
  if [[ -n "${KC_ADMIN_USER_ID}" ]]; then
    return 0
  fi

  local admin_query
  admin_query=$(kcadm get users -r "${KC_ADMIN_REALM}" -q username="${KC_ADMIN}" 2>/dev/null || true)
  KC_ADMIN_USER_ID=$(printf '%s' "${admin_query}" | resolve_keycloak_first_id)

  if [[ -z "${KC_ADMIN_USER_ID}" ]]; then
    log "Unable to resolve Keycloak admin user id for ${KC_ADMIN}." >&2
    exit 1
  fi
}

ensure_oidc_client() {
  local site_base="${MM_SITE_URL%/}"
  OIDC_REDIRECT_URL="${site_base}${OIDC_REDIRECT_PATH}"
  local logout_redirect="${site_base}/plugins/${PLUGIN_ID}"
  local default_issuer="http://${KC_HOSTNAME}:${KC_HTTP_PORT}/realms/${OIDC_REALM}"
  OIDC_EFFECTIVE_ISSUER="${OIDC_ISSUER_URL:-${default_issuer}}"

  local client_query
  client_query=$(kcadm get clients -r "${OIDC_REALM}" -q clientId="${OIDC_CLIENT_ID}" 2>/dev/null || true)
  local client_internal_id
  client_internal_id=$(printf '%s' "${client_query}" | resolve_keycloak_first_id)

  if [[ -z "${client_internal_id}" ]]; then
    log "Creating Keycloak client ${OIDC_CLIENT_ID} in realm ${OIDC_REALM}."
    kcadm create clients -r "${OIDC_REALM}" \
      -s "clientId=${OIDC_CLIENT_ID}" \
      -s "name=${OIDC_CLIENT_NAME}" \
      -s protocol=openid-connect \
      -s publicClient=false \
      -s standardFlowEnabled=true \
      -s implicitFlowEnabled=false \
      -s directAccessGrantsEnabled=false \
      -s serviceAccountsEnabled=false \
      -s 'redirectUris=["'"${OIDC_REDIRECT_URL}"'"]' \
      -s 'webOrigins=["'"${site_base}"'"]' \
      -s 'attributes."post.logout.redirect.uris"="'"${logout_redirect}"'"' >/dev/null

    client_query=$(kcadm get clients -r "${OIDC_REALM}" -q clientId="${OIDC_CLIENT_ID}" 2>/dev/null || true)
    client_internal_id=$(printf '%s' "${client_query}" | resolve_keycloak_first_id)
  else
    log "Updating Keycloak client ${OIDC_CLIENT_ID} redirect/web origins."
    kcadm update clients/"${client_internal_id}" -r "${OIDC_REALM}" \
      -s "name=${OIDC_CLIENT_NAME}" \
      -s publicClient=false \
      -s standardFlowEnabled=true \
      -s implicitFlowEnabled=false \
      -s directAccessGrantsEnabled=false \
      -s 'redirectUris=["'"${OIDC_REDIRECT_URL}"'"]' \
      -s 'webOrigins=["'"${site_base}"'"]' \
      -s 'attributes."post.logout.redirect.uris"="'"${logout_redirect}"'"' >/dev/null
  fi

  if [[ -z "${client_internal_id}" ]]; then
    log "Unable to resolve Keycloak client id for ${OIDC_CLIENT_ID}." >&2
    exit 1
  fi

  OIDC_CLIENT_INTERNAL_ID="${client_internal_id}"

  OIDC_CLIENT_SECRET=$(kcadm get clients/"${client_internal_id}"/client-secret -r "${OIDC_REALM}" | \
    python3 -c 'import json, sys; data = json.load(sys.stdin); print(data.get("value", ""))')

  if [[ -z "${OIDC_CLIENT_SECRET}" ]]; then
    log "Failed to read client secret for ${OIDC_CLIENT_ID}." >&2
    exit 1
  fi

  log "Keycloak client ${OIDC_CLIENT_ID} ready with redirect ${OIDC_REDIRECT_URL}."
}

ensure_client_admin_role() {
  if [[ -z "${OIDC_CLIENT_ADMIN_ROLE}" ]]; then
    log "OIDC_CLIENT_ADMIN_ROLE is empty; skipping client role provisioning."
    return 0
  fi

  if [[ -z "${OIDC_CLIENT_INTERNAL_ID}" ]]; then
    log "Client internal id unavailable; cannot manage client roles." >&2
    exit 1
  fi

  if kcadm get clients/"${OIDC_CLIENT_INTERNAL_ID}"/roles/"${OIDC_CLIENT_ADMIN_ROLE}" -r "${OIDC_REALM}" >/dev/null 2>&1; then
    log "Keycloak client role ${OIDC_CLIENT_ADMIN_ROLE} already exists for ${OIDC_CLIENT_ID}."
    return 0
  fi

  log "Creating Keycloak client role ${OIDC_CLIENT_ADMIN_ROLE} for ${OIDC_CLIENT_ID}."
  kcadm create clients/"${OIDC_CLIENT_INTERNAL_ID}"/roles -r "${OIDC_REALM}" \
    -s "name=${OIDC_CLIENT_ADMIN_ROLE}" \
    -s 'description=Mattermost system admin bridge role' >/dev/null
}

ensure_admin_has_client_role() {
  if [[ -z "${OIDC_CLIENT_ADMIN_ROLE}" ]]; then
    return 0
  fi

  ensure_admin_user_id_cached

  if [[ -z "${OIDC_CLIENT_INTERNAL_ID}" ]]; then
    log "Client internal id unavailable; cannot assign admin role." >&2
    exit 1
  fi

  local mapping
  mapping=$(kcadm get users/"${KC_ADMIN_USER_ID}"/role-mappings/clients/"${OIDC_CLIENT_INTERNAL_ID}" -r "${OIDC_REALM}" 2>/dev/null || true)
  if printf '%s' "${mapping}" | TARGET_ROLE="${OIDC_CLIENT_ADMIN_ROLE}" python3 -c 'import json, os, sys
raw = sys.stdin.read().strip()
if not raw:
  sys.exit(1)
try:
  data = json.loads(raw)
except json.JSONDecodeError:
  sys.exit(1)
target = os.environ.get("TARGET_ROLE", "")
for item in data:
  if isinstance(item, dict) and item.get("name") == target:
    sys.exit(0)
sys.exit(1)
'; then
    log "Keycloak admin ${KC_ADMIN} already has client role ${OIDC_CLIENT_ADMIN_ROLE}."
    return 0
  fi

  log "Granting client role ${OIDC_CLIENT_ADMIN_ROLE} to Keycloak admin ${KC_ADMIN}."
  kcadm add-roles -r "${OIDC_REALM}" \
    --uusername "${KC_ADMIN}" \
    --cclientid "${OIDC_CLIENT_ID}" \
    --rolename "${OIDC_CLIENT_ADMIN_ROLE}" >/dev/null
}

client_mapper_exists() {
  local mapper_name="${1:-}"
  if [[ -z "${mapper_name}" ]]; then
    return 1
  fi

  if [[ -z "${OIDC_CLIENT_INTERNAL_ID}" ]]; then
    return 1
  fi

  local payload
  payload=$(kcadm get clients/"${OIDC_CLIENT_INTERNAL_ID}"/protocol-mappers/models -r "${OIDC_REALM}" 2>/dev/null || true)
  if [[ -z "${payload}" ]]; then
    return 1
  fi

  if printf '%s' "${payload}" | TARGET_NAME="${mapper_name}" python3 -c 'import json, os, sys
raw = sys.stdin.read().strip()
if not raw:
  sys.exit(1)
try:
  data = json.loads(raw)
except json.JSONDecodeError:
  sys.exit(1)
target = os.environ.get("TARGET_NAME", "")
for item in data:
  if isinstance(item, dict) and item.get("name") == target:
    sys.exit(0)
sys.exit(1)
'; then
    return 0
  fi

  return 1
}

ensure_client_mapper() {
  local mapper_name="${1:-}"
  shift || true

  if [[ -z "${mapper_name}" ]]; then
    log "Mapper name missing; aborting." >&2
    exit 1
  fi

  if [[ -z "${OIDC_CLIENT_INTERNAL_ID}" ]]; then
    log "Client internal id unavailable; cannot manage protocol mappers." >&2
    exit 1
  fi

  if client_mapper_exists "${mapper_name}"; then
    log "Keycloak client mapper ${mapper_name} already exists."
    return 0
  fi

  log "Creating Keycloak client mapper ${mapper_name}."
  kcadm create clients/"${OIDC_CLIENT_INTERNAL_ID}"/protocol-mappers/models -r "${OIDC_REALM}" \
    -s "name=${mapper_name}" \
    "$@"
}

ensure_oidc_client_mappers() {
  ensure_client_mapper "preferred_username" \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-usermodel-property-mapper \
    -s 'config."userinfo.token.claim"=true' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."user.attribute"=username' \
    -s 'config."claim.name"=preferred_username' \
    -s 'config."jsonType.label"=String'

  ensure_client_mapper "given_name" \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-usermodel-property-mapper \
    -s 'config."userinfo.token.claim"=true' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."user.attribute"=firstName' \
    -s 'config."claim.name"=given_name' \
    -s 'config."jsonType.label"=String'

  ensure_client_mapper "family_name" \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-usermodel-property-mapper \
    -s 'config."userinfo.token.claim"=true' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."user.attribute"=lastName' \
    -s 'config."claim.name"=family_name' \
    -s 'config."jsonType.label"=String'

  ensure_client_mapper "full_name" \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-full-name-mapper \
    -s 'config."userinfo.token.claim"=true' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true'

  ensure_client_mapper "email" \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-usermodel-property-mapper \
    -s 'config."userinfo.token.claim"=true' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."user.attribute"=email' \
    -s 'config."claim.name"=email' \
    -s 'config."jsonType.label"=String'

  ensure_client_mapper "mm-oidc-client-roles" \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-usermodel-client-role-mapper \
    -s 'config."userinfo.token.claim"=true' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."multivalued"=true' \
    -s 'config."jsonType.label"=String' \
    -s 'config."usermodel.clientRoleMapping.rolePrefix"=' \
    -s "config.\"usermodel.clientRoleMapping.clientId\"=${OIDC_CLIENT_ID}" \
    -s "config.\"claim.name\"=resource_access.${OIDC_CLIENT_ID}.roles"
}

configure_plugin_settings() {
  if [[ -z "${OIDC_CLIENT_SECRET}" ]]; then
    log "Missing Keycloak client secret; cannot configure plugin." >&2
    exit 1
  fi

  local allow_insecure
  allow_insecure=$(printf '%s' "${OIDC_ALLOW_INSECURE}" | tr '[:upper:]' '[:lower:]')
  local patch_payload
  patch_payload=$(PLUGIN_KEY="${PLUGIN_ID}" \
    ISSUER="${OIDC_EFFECTIVE_ISSUER}" \
    ALLOW_INSECURE="${allow_insecure}" \
    CLIENT_ID="${OIDC_CLIENT_ID}" \
    CLIENT_SECRET="${OIDC_CLIENT_SECRET}" \
    REDIRECT_URL="${OIDC_REDIRECT_URL}" \
    SCOPE_INPUT="${OIDC_SCOPES}" \
    python3 -c '
import json, os
scopes = [token for token in os.environ["SCOPE_INPUT"].replace(",", " ").split() if token]
if not scopes:
    scopes = ["openid"]
doc = {
    "PluginSettings": {
        "Plugins": {
            os.environ["PLUGIN_KEY"]: {
                "issuer_url": os.environ["ISSUER"],
                "allow_insecure_issuer": os.environ["ALLOW_INSECURE"].lower() in ("true", "1", "yes"),
                "client_id": os.environ["CLIENT_ID"],
                "client_secret": os.environ["CLIENT_SECRET"],
                "redirect_url": os.environ["REDIRECT_URL"],
                "scopes": scopes,
            }
        }
    }
}
print(json.dumps(doc))
')

  local patch_remote="/tmp/mm-oidc-plugin-config.json"
  local patch_host
  patch_host=$(mktemp)
  printf '%s\n' "${patch_payload}" >"${patch_host}"
  chmod 644 "${patch_host}"
  docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" cp "${patch_host}" mattermost:"${patch_remote}"
  rm -f "${patch_host}"

  log "Updating plugin configuration for ${PLUGIN_ID}."
  run_mmctl config patch "${patch_remote}" >/dev/null

  log "Restarting plugin ${PLUGIN_ID} to pick up new settings."
  run_mmctl plugin disable "${PLUGIN_ID}" >/dev/null || true
  run_mmctl plugin enable "${PLUGIN_ID}" >/dev/null
}

main() {
  if [[ ! -f "${ENV_FILE}" ]]; then
    log "Missing env file at ${ENV_FILE}. Run scripts/dev-up.sh first." >&2
    exit 1
  fi

  mkdir -p "${PLUGINS_DIR}"

  # shellcheck disable=SC1090
  source "${ENV_FILE}"

  : "${MM_ADMIN_USERNAME:?MM_ADMIN_USERNAME must be set in ${ENV_FILE}}"
  : "${MM_ADMIN_EMAIL:?MM_ADMIN_EMAIL must be set in ${ENV_FILE}}"
  : "${MM_ADMIN_PASSWORD:?MM_ADMIN_PASSWORD must be set in ${ENV_FILE}}"
  : "${MM_SITE_URL:?MM_SITE_URL must be set in ${ENV_FILE}}"
  : "${KC_HTTP_PORT:?KC_HTTP_PORT must be set in ${ENV_FILE}}"
  : "${KC_ADMIN:?KC_ADMIN must be set in ${ENV_FILE}}"
  : "${KC_ADMIN_PASSWORD:?KC_ADMIN_PASSWORD must be set in ${ENV_FILE}}"
  : "${KC_ADMIN_EMAIL:?KC_ADMIN_EMAIL must be set in ${ENV_FILE}}"
  : "${OIDC_REALM:?OIDC_REALM must be set in ${ENV_FILE}}"
  : "${OIDC_CLIENT_ID:?OIDC_CLIENT_ID must be set in ${ENV_FILE}}"
  : "${OIDC_CLIENT_NAME:?OIDC_CLIENT_NAME must be set in ${ENV_FILE}}"
  : "${OIDC_SCOPES:?OIDC_SCOPES must be set in ${ENV_FILE}}"
  : "${OIDC_REDIRECT_PATH:?OIDC_REDIRECT_PATH must be set in ${ENV_FILE}}"

  KC_ADMIN_REALM="${KC_ADMIN_REALM:-master}"
  KC_HOSTNAME="${KC_HOSTNAME:-localhost}"
  OIDC_ALLOW_INSECURE="${OIDC_ALLOW_INSECURE:-false}"
  OIDC_CLIENT_ADMIN_ROLE="${OIDC_CLIENT_ADMIN_ROLE:-system_admin}"

  if [[ "${OIDC_REDIRECT_PATH}" != /* ]]; then
    OIDC_REDIRECT_PATH="/${OIDC_REDIRECT_PATH}"
  fi

  build_plugin
  wait_for_keycloak
  keycloak_login
  ensure_keycloak_admin_email
  ensure_oidc_realm
  ensure_oidc_client
  ensure_oidc_client_mappers
  ensure_client_admin_role
  ensure_admin_has_client_role
  wait_for_mattermost
  ensure_admin_user
  ensure_plugin_installed
  configure_plugin_settings
}

main "$@"
