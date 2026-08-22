#!/usr/bin/env bash
# Restore the latest encrypted Velora + Casdoor backup into isolated temporary
# databases, compare key row counts, emit evidence, then remove the databases.
set -euo pipefail
umask 077

RUNTIME_DIR="${VELORA_RUNTIME_DIR:-/opt/velora/prod/runtime}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-prod-postgres}"
BACKUP_STAMP="${1:-}"
if [[ -z "$BACKUP_STAMP" ]]; then
  latest="$(find "$RUNTIME_DIR/backups" -maxdepth 1 -type f -name 'velora_full_*.dump.age' -print | sort | tail -1)"
  [[ -n "$latest" ]] || { echo "error: no encrypted Velora backup found" >&2; exit 1; }
  BACKUP_STAMP="${latest##*velora_full_}"
  BACKUP_STAMP="${BACKUP_STAMP%.dump.age}"
fi
[[ "$BACKUP_STAMP" =~ ^[0-9]{8}_[0-9]{6}$ ]] || { echo "error: invalid backup stamp" >&2; exit 1; }
[[ "$POSTGRES_CONTAINER" =~ ^[a-zA-Z0-9_.-]+$ ]] || { echo "error: invalid PostgreSQL container" >&2; exit 1; }

for tool in docker age openssl jq sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: required restore tool is missing: $tool" >&2; exit 1; }
done

identity="$RUNTIME_DIR/secrets/backup-age-identity"
public_key="$RUNTIME_DIR/secrets/backup-signing.pub"
[[ -r "$identity" && -r "$public_key" ]] || { echo "error: restore verification keys are unavailable" >&2; exit 1; }

suffix="${BACKUP_STAMP//_/}"
velora_restore="velora_restore_${suffix}"
casdoor_restore="casdoor_restore_${suffix}"
tmp_dir="$(mktemp -d)"

db_exec() {
  docker exec "$POSTGRES_CONTAINER" sh -ec 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1" -Atc "$2"' sh "$1" "$2"
}

cleanup() {
  docker exec "$POSTGRES_CONTAINER" rm -f "/tmp/${velora_restore}.dump" "/tmp/${casdoor_restore}.dump" >/dev/null 2>&1 || true
  for database in "$velora_restore" "$casdoor_restore"; do
    if [[ "$database" =~ ^(velora|casdoor)_restore_[0-9]{14}$ ]]; then
      docker exec "$POSTGRES_CONTAINER" sh -ec 'dropdb --if-exists --force -U "$POSTGRES_USER" "$1"' sh "$database" >/dev/null 2>&1 || true
    fi
  done
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

for database in velora casdoor; do
  encrypted="$RUNTIME_DIR/backups/${database}_full_${BACKUP_STAMP}.dump.age"
  manifest="${encrypted}.sha256"
  signature="${encrypted}.sig"
  [[ -s "$encrypted" && -s "$manifest" && -s "$signature" ]] || { echo "error: incomplete backup set for $database" >&2; exit 1; }
  (cd / && sha256sum -c "$manifest" >/dev/null)
  openssl dgst -sha256 -verify "$public_key" -signature "$signature" "$encrypted" >/dev/null
  age --decrypt -i "$identity" -o "$tmp_dir/${database}.dump" "$encrypted"
done

for database in "$velora_restore" "$casdoor_restore"; do
  db_exec postgres "CREATE DATABASE ${database};" >/dev/null
done
docker cp "$tmp_dir/velora.dump" "$POSTGRES_CONTAINER:/tmp/${velora_restore}.dump" >/dev/null
docker cp "$tmp_dir/casdoor.dump" "$POSTGRES_CONTAINER:/tmp/${casdoor_restore}.dump" >/dev/null
docker exec "$POSTGRES_CONTAINER" sh -ec 'pg_restore -U "$POSTGRES_USER" -d "$1" --no-owner --no-acl "$2"' sh "$velora_restore" "/tmp/${velora_restore}.dump" >/dev/null
docker exec "$POSTGRES_CONTAINER" sh -ec 'pg_restore -U "$POSTGRES_USER" -d "$1" --no-owner --no-acl "$2"' sh "$casdoor_restore" "/tmp/${casdoor_restore}.dump" >/dev/null

velora_source="$(db_exec velora "SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM portal_applications),(SELECT count(*) FROM audit_logs);")"
velora_restored="$(db_exec "$velora_restore" "SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM portal_applications),(SELECT count(*) FROM audit_logs);")"
casdoor_source="$(db_exec casdoor 'SELECT (SELECT count(*) FROM "user"),(SELECT count(*) FROM application),(SELECT count(*) FROM token);')"
casdoor_restored="$(db_exec "$casdoor_restore" 'SELECT (SELECT count(*) FROM "user"),(SELECT count(*) FROM application),(SELECT count(*) FROM token);')"
[[ "$velora_source" == "$velora_restored" && "$casdoor_source" == "$casdoor_restored" ]] || { echo "error: restored key row counts do not match source" >&2; exit 1; }

cleanup
trap - EXIT

evidence="$RUNTIME_DIR/evidence/backup-restore-${BACKUP_STAMP}.json"
tmp_evidence="${evidence}.tmp"
jq -n \
  --arg completed_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg backup_stamp "$BACKUP_STAMP" \
  --arg velora_counts "$velora_restored" \
  --arg casdoor_counts "$casdoor_restored" \
  '{schema:"velora.backup.restore.v1",status:"passed",completed_at:$completed_at,backup_stamp:$backup_stamp,isolated_restore:true,key_counts:{velora:$velora_counts,casdoor:$casdoor_counts},temporary_databases_removed:true}' \
  >"$tmp_evidence"
chmod 600 "$tmp_evidence"
mv -f "$tmp_evidence" "$evidence"
echo "backup restore verification passed: $evidence"
