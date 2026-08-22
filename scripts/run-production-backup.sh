#!/usr/bin/env bash
# Root-only production wrapper: consistent Velora + Casdoor dump, encryption,
# signature and Tencent COS upload. Never prints secret values.
set -euo pipefail
umask 077

RUNTIME_DIR="${VELORA_RUNTIME_DIR:-/opt/velora/prod/runtime}"
ENV_FILE="${VELORA_PROD_ENV_FILE:-$RUNTIME_DIR/compose/prod.env}"
STATUS_FILE="${VELORA_BACKUP_STATUS_FILE:-$RUNTIME_DIR/evidence/backup-last-success.json}"

if [[ ! -r "$ENV_FILE" ]]; then
  echo "error: production environment file is not readable" >&2
  exit 1
fi
set -a
# shellcheck disable=SC1090 -- the root-owned deployment environment is the authority.
source "$ENV_FILE"
set +a

for tool in docker age openssl jq; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: required backup tool is missing: $tool" >&2; exit 1; }
done
for path in \
  "$VELORA_DATABASE_DSN_FILE" "$POSTGRES_IDP_PASSWORD_FILE" \
  "$VELORA_STORAGE_ACCESS_KEY_FILE" "$VELORA_STORAGE_SECRET_KEY_FILE" \
  "$RUNTIME_DIR/secrets/backup-age-recipient" "$RUNTIME_DIR/secrets/backup-signing.key"; do
  [[ -r "$path" ]] || { echo "error: required backup secret is not readable" >&2; exit 1; }
done

idp_password_encoded="$(jq -rn --arg value "$(<"$POSTGRES_IDP_PASSWORD_FILE")" '$value|@uri')"
velora_database_url="$(<"$VELORA_DATABASE_DSN_FILE")"
casdoor_database_url="postgres://${POSTGRES_IDP_USER}:${idp_password_encoded}@postgres:5432/casdoor?sslmode=require"
unset idp_password_encoded

export AWS_ACCESS_KEY_ID="$(<"$VELORA_STORAGE_ACCESS_KEY_FILE")"
export AWS_SECRET_ACCESS_KEY="$(<"$VELORA_STORAGE_SECRET_KEY_FILE")"
export AWS_DEFAULT_REGION="$VELORA_STORAGE_REGION"
export AWS_ENDPOINT_URL="$VELORA_STORAGE_ENDPOINT"
export AWS_EC2_METADATA_DISABLED=true

stamp="$(date -u +%Y%m%d_%H%M%S)"
backup_dir="$RUNTIME_DIR/backups"
mkdir -p "$backup_dir" "$(dirname "$STATUS_FILE")"

DATABASE_URL="$velora_database_url" \
CASDOOR_DATABASE_URL="$casdoor_database_url" \
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-prod-postgres}" \
BACKUP_USE_CONTAINER=true \
BACKUP_DATABASE_HOST=postgres \
BACKUP_DIR="$backup_dir" \
BACKUP_STAMP="$stamp" \
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}" \
BACKUP_ENCRYPTION_REQUIRED=true \
BACKUP_ENCRYPTION_KEY_FILE="$RUNTIME_DIR/secrets/backup-age-recipient" \
BACKUP_SIGNING_REQUIRED=true \
BACKUP_SIGNING_KEY_FILE="$RUNTIME_DIR/secrets/backup-signing.key" \
BACKUP_S3="s3://${VELORA_STORAGE_BUCKET}/${VELORA_STORAGE_PREFIX%/}/backups" \
  "$(dirname "$0")/backup-all-databases.sh"

unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY velora_database_url casdoor_database_url
tmp_status="${STATUS_FILE}.tmp"
jq -n --arg completed_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg stamp "$stamp" \
  '{schema:"velora.backup.status.v1",status:"passed",completed_at:$completed_at,backup_stamp:$stamp,databases:["velora","casdoor"],encrypted:true,signed:true,remote_copy:true}' \
  >"$tmp_status"
chmod 600 "$tmp_status"
mv -f "$tmp_status" "$STATUS_FILE"
