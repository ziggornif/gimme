#!/bin/sh
# init-garage.sh — Provisions a single-node Garage cluster and writes gimme.yml.
# Runs as a one-shot Alpine container; communicates with Garage via the Admin HTTP API v1 (port 3903).
# This script is idempotent: safe to re-run after a docker compose restart.
set -e

# Install dependencies (curl + jq are not pre-installed in alpine:3.x)
apk add --no-cache curl jq > /dev/null 2>&1

GARAGE_HOST="${GARAGE_HOST:-garage}"
GARAGE_S3_PORT="${GARAGE_S3_PORT:-3900}"
GARAGE_ADMIN_PORT="${GARAGE_ADMIN_PORT:-3903}"
# Must match admin_token in garage.toml
GARAGE_ADMIN_TOKEN="${GARAGE_ADMIN_TOKEN:-gimme-init-token-change-me}"
GIMME_CONFIG_PATH="${GIMME_CONFIG_PATH:-/gimme-config/gimme.yml}"
KEY_NAME="${KEY_NAME:-gimme-key}"
BUCKET_NAME="${BUCKET_NAME:-gimme}"
# Capacity in bytes (default: 10 GiB = 10737418240)
GARAGE_CAPACITY_BYTES="${GARAGE_CAPACITY_BYTES:-10737418240}"
GARAGE_ZONE="${GARAGE_ZONE:-dc1}"
GARAGE_REGION="${GARAGE_REGION:-garage}"

ADMIN_BASE="http://${GARAGE_HOST}:${GARAGE_ADMIN_PORT}"

log() { echo "[init-garage] $*"; }

curl_admin() {
  curl -sf -H "Content-Type: application/json" \
       -H "Authorization: Bearer ${GARAGE_ADMIN_TOKEN}" \
       "$@"
}

# ── Wait for Garage ───────────────────────────────────────────────────────────
# NOTE: init-garage depends_on garage with service_healthy, so Garage is already
# up when this script runs. The loop below is a safety net for timing edge cases.

log "Waiting for Garage Admin API at ${ADMIN_BASE}/health ..."
RETRIES=15
until curl_admin "${ADMIN_BASE}/health" > /dev/null 2>&1 || [ "$RETRIES" -eq 0 ]; do
  sleep 2
  RETRIES=$(( RETRIES - 1 ))
done
if [ "$RETRIES" -eq 0 ]; then
  log "ERROR: Garage Admin API did not become ready after 30s."
  exit 1
fi
log "Garage Admin API is up."

# ── Layout ────────────────────────────────────────────────────────────────────

LAYOUT_INFO=$(curl_admin "${ADMIN_BASE}/v1/layout")
LAYOUT_VERSION=$(echo "$LAYOUT_INFO" | jq -r '.version')
# grep -c returns exit 1 (no matches) which would abort under set -e — use jq instead
LAYOUT_ROLES=$(echo "$LAYOUT_INFO" | jq '.roles | length')

if [ "${LAYOUT_ROLES:-0}" -eq 0 ]; then
  log "Assigning cluster layout..."

  NODE_ID=$(curl_admin "${ADMIN_BASE}/v1/status" | jq -r '.nodes[0].id')
  if [ -z "$NODE_ID" ] || [ "$NODE_ID" = "null" ]; then
    log "ERROR: Could not get node ID from Garage status API."
    exit 1
  fi
  log "Node ID: ${NODE_ID}"

  # Stage role assignment
  curl_admin -X POST "${ADMIN_BASE}/v1/layout" \
    -d "[{\"id\":\"${NODE_ID}\",\"zone\":\"${GARAGE_ZONE}\",\"capacity\":${GARAGE_CAPACITY_BYTES},\"tags\":[]}]" \
    > /dev/null

  # Apply layout (version = current + 1)
  NEXT_VERSION=$(( ${LAYOUT_VERSION:-0} + 1 ))
  curl_admin -X POST "${ADMIN_BASE}/v1/layout/apply" \
    -d "{\"version\":${NEXT_VERSION}}" > /dev/null

  log "Layout applied (version ${NEXT_VERSION})."
else
  log "Layout already configured (version ${LAYOUT_VERSION}), skipping."
fi

# ── Bucket ────────────────────────────────────────────────────────────────────

BUCKET_LIST=$(curl_admin "${ADMIN_BASE}/v1/bucket?list")
# Use jq with @sh to safely handle special characters in BUCKET_NAME
BUCKET_ID=$(echo "$BUCKET_LIST" | jq -r --arg name "$BUCKET_NAME" \
  '.[] | select(.globalAliases[] == $name) | .id' 2>/dev/null | head -1)

if [ -n "$BUCKET_ID" ]; then
  log "Bucket '${BUCKET_NAME}' already exists (id=${BUCKET_ID})."
else
  log "Creating bucket '${BUCKET_NAME}'..."
  BUCKET_RESP=$(curl_admin -X POST "${ADMIN_BASE}/v1/bucket" \
    -d "{\"globalAlias\":\"${BUCKET_NAME}\"}")
  BUCKET_ID=$(echo "$BUCKET_RESP" | jq -r '.id')
  log "Bucket created (id=${BUCKET_ID})."
fi

if [ -z "$BUCKET_ID" ] || [ "$BUCKET_ID" = "null" ]; then
  log "ERROR: Could not determine bucket ID."
  exit 1
fi

# ── Key ───────────────────────────────────────────────────────────────────────

KEY_LIST=$(curl_admin "${ADMIN_BASE}/v1/key?list")
KEY_ID=$(echo "$KEY_LIST" | jq -r --arg name "$KEY_NAME" \
  '.[] | select(.name == $name) | .id' 2>/dev/null | head -1)

if [ -n "$KEY_ID" ]; then
  log "Key '${KEY_NAME}' already exists (id=${KEY_ID})."
else
  log "Creating S3 key '${KEY_NAME}'..."
  KEY_RESP=$(curl_admin -X POST "${ADMIN_BASE}/v1/key" \
    -d "{\"name\":\"${KEY_NAME}\"}")
  KEY_ID=$(echo "$KEY_RESP" | jq -r '.accessKeyId')
  log "Key created (id=${KEY_ID})."
fi

if [ -z "$KEY_ID" ] || [ "$KEY_ID" = "null" ]; then
  log "ERROR: Could not determine key ID."
  exit 1
fi

# ── Grant permissions ─────────────────────────────────────────────────────────

# Check if permissions are already granted to avoid misleading "Granting" log on re-runs
ALREADY_GRANTED=$(curl_admin "${ADMIN_BASE}/v1/key?id=${KEY_ID}" | \
  jq -r --arg bucket "$BUCKET_ID" \
  '.buckets[] | select(.id == $bucket) | .permissions | .read and .write and .owner' 2>/dev/null || echo "false")

if [ "$ALREADY_GRANTED" = "true" ]; then
  log "Permissions on '${BUCKET_NAME}' for key '${KEY_ID}' already granted."
else
  log "Granting read/write/owner on '${BUCKET_NAME}' to key '${KEY_ID}'..."
  curl_admin -X POST "${ADMIN_BASE}/v1/bucket/allow" \
    -d "{\"bucketId\":\"${BUCKET_ID}\",\"accessKeyId\":\"${KEY_ID}\",\"permissions\":{\"read\":true,\"write\":true,\"owner\":true}}" \
    > /dev/null
  log "Permissions granted."
fi

# ── Fetch secret key ──────────────────────────────────────────────────────────

KEY_INFO=$(curl_admin "${ADMIN_BASE}/v1/key?id=${KEY_ID}&showSecretKey=true")
# Use jq to extract the secret — avoids fragile regex assumptions about key format
SECRET_KEY=$(echo "$KEY_INFO" | jq -r '.secretAccessKey')

if [ -z "$SECRET_KEY" ] || [ "$SECRET_KEY" = "null" ]; then
  log "ERROR: Could not retrieve secret key."
  exit 1
fi

# ── Write gimme.yml ───────────────────────────────────────────────────────────

CACHE_ENABLED="${CACHE_ENABLED:-true}"
CACHE_REDIS_URL="${CACHE_REDIS_URL:-redis://redis:6379}"
CACHE_TTL="${CACHE_TTL:-3600}"

log "Writing ${GIMME_CONFIG_PATH}..."
mkdir -p "$(dirname "$GIMME_CONFIG_PATH")"
cat > "$GIMME_CONFIG_PATH" <<EOF
admin:
  user: ${GIMME_ADMIN_USER:-gimmeadmin}
  password: ${GIMME_ADMIN_PASSWORD:-gimmeadmin}
port: ${GIMME_PORT:-8080}
secret: ${GIMME_SECRET:-change_me_use_a_real_secret}
s3:
  url: ${GARAGE_HOST}:${GARAGE_S3_PORT}
  key: ${KEY_ID}
  secret: ${SECRET_KEY}
  bucketName: ${BUCKET_NAME}
  location: ${GARAGE_REGION}
  ssl: false
metrics: true
cache:
  enabled: ${CACHE_ENABLED}
  type: redis
  ttl: ${CACHE_TTL}
  redis_url: ${CACHE_REDIS_URL}
EOF

log "Done."
log "  s3.key = ${KEY_ID}"
log "  gimme config written to ${GIMME_CONFIG_PATH}"
