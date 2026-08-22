#!/usr/bin/env bash
# Root-only standard-profile audit snapshot: export, encrypt, sign and upload
# to Tencent COS. This is an independent recoverable copy, not a WORM claim.
set -euo pipefail
umask 077

RUNTIME_DIR="${VELORA_RUNTIME_DIR:-/opt/velora/prod/runtime}"
ENV_FILE="${VELORA_PROD_ENV_FILE:-$RUNTIME_DIR/compose/prod.env}"
STATUS_FILE="${VELORA_AUDIT_ARCHIVE_STATUS_FILE:-$RUNTIME_DIR/evidence/audit-archive-last-success.json}"
LOCAL_DIR="${VELORA_AUDIT_ARCHIVE_DIR:-$RUNTIME_DIR/audit-archive}"

[[ -r "$ENV_FILE" ]] || { echo "error: production environment file is not readable" >&2; exit 1; }
set -a
# shellcheck disable=SC1090 -- root-owned deployment environment is authoritative.
source "$ENV_FILE"
set +a

for tool in docker age openssl jq aws tar sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: required audit archive tool is missing: $tool" >&2; exit 1; }
done
for path in \
  "$VELORA_DATABASE_DSN_FILE" "$VELORA_STORAGE_ACCESS_KEY_FILE" \
  "$VELORA_STORAGE_SECRET_KEY_FILE" "$RUNTIME_DIR/secrets/backup-age-recipient" \
  "$RUNTIME_DIR/secrets/backup-signing.key"; do
  [[ -r "$path" ]] || { echo "error: required audit archive secret is not readable" >&2; exit 1; }
done

mkdir -p "$LOCAL_DIR" "$(dirname "$STATUS_FILE")"
chmod 700 "$LOCAL_DIR"
work_dir="$(mktemp -d "$LOCAL_DIR/.work.XXXXXX")"
aws_config="$(mktemp)"
cleanup() {
  find "$work_dir" -type f -delete 2>/dev/null || true
  rmdir "$work_dir" 2>/dev/null || true
  [[ ! -e "$aws_config" ]] || unlink "$aws_config"
}
trap cleanup EXIT

export AWS_ACCESS_KEY_ID="$(<"$VELORA_STORAGE_ACCESS_KEY_FILE")"
export AWS_SECRET_ACCESS_KEY="$(<"$VELORA_STORAGE_SECRET_KEY_FILE")"
export AWS_DEFAULT_REGION="$VELORA_STORAGE_REGION"
export AWS_ENDPOINT_URL="$VELORA_STORAGE_ENDPOINT"
export AWS_EC2_METADATA_DISABLED=true
cat >"$aws_config" <<EOF
[default]
region = $VELORA_STORAGE_REGION
s3 =
    addressing_style = virtual
EOF
export AWS_CONFIG_FILE="$aws_config"

stamp="$(date -u +%Y%m%d_%H%M%S)"
cutoff="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
DATABASE_URL="$(<"$VELORA_DATABASE_DSN_FILE")" \
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-prod-postgres}" \
AUDIT_ARCHIVE_DIR="$work_dir" \
AUDIT_EXPORT_CUTOFF="$cutoff" \
  "$(dirname "$0")/audit-archive.sh"

csv="$(find "$work_dir" -maxdepth 1 -type f -name 'audit_until_*.csv' -print -quit)"
[[ -n "$csv" && -s "$csv" ]] || { echo "error: audit CSV was not produced" >&2; exit 1; }
metadata="${csv}.metadata"
event_count="$(awk -F= '$1=="event_count" {print $2}' "$metadata")"
[[ "$event_count" =~ ^[0-9]+$ ]] || { echo "error: audit archive event count is invalid" >&2; exit 1; }

raw="$work_dir/audit_${stamp}.tar"
encrypted="$LOCAL_DIR/audit_${stamp}.tar.age"
tar -C "$work_dir" -cf "$raw" "$(basename "$csv")" "$(basename "$csv").sha256" "$(basename "$metadata")"
age -R "$RUNTIME_DIR/secrets/backup-age-recipient" -o "$encrypted" "$raw"
manifest="${encrypted}.sha256"
signature="${encrypted}.sig"
sha256sum "$encrypted" >"$manifest"
openssl dgst -sha256 -sign "$RUNTIME_DIR/secrets/backup-signing.key" -out "$signature" "$encrypted"

prefix="${VELORA_STORAGE_PREFIX%/}/audit-standard"
for file in "$encrypted" "$manifest" "$signature"; do
  aws s3 cp "$file" "s3://${VELORA_STORAGE_BUCKET}/${prefix}/$(basename "$file")" --only-show-errors
done
object_key="${prefix}/$(basename "$encrypted")"
aws s3api head-object --bucket "$VELORA_STORAGE_BUCKET" --key "$object_key" >/dev/null

tmp_status="${STATUS_FILE}.tmp"
jq -n \
  --arg completed_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg cutoff "$cutoff" \
  --arg bucket "$VELORA_STORAGE_BUCKET" \
  --arg object_key "$object_key" \
  --argjson event_count "$event_count" \
  '{schema:"velora.audit.archive.status.v1",status:"passed",profile:"standard",completed_at:$completed_at,cutoff:$cutoff,bucket:$bucket,object_key:$object_key,event_count:$event_count,encrypted:true,signed:true,remote_copy:true,immutable:false}' \
  >"$tmp_status"
chmod 600 "$tmp_status"
mv -f "$tmp_status" "$STATUS_FILE"

find "$LOCAL_DIR" -maxdepth 1 -type f -name 'audit_*.tar.age*' -mtime "+${AUDIT_ARCHIVE_LOCAL_RETENTION_DAYS:-30}" -delete
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_CONFIG_FILE
echo "production audit archive passed"
