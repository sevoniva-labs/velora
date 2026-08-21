#!/usr/bin/env bash
# 本地双库备份、WAL 归档和 PITR 恢复演练。
# 该脚本只使用随机 Compose project、临时卷和临时恢复库，不接触现有开发数据库。
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

for binary in docker psql pg_dump pg_restore pg_isready; do
  command -v "$binary" >/dev/null 2>&1 || { echo "错误：缺少 $binary" >&2; exit 2; }
done

port="${PITR_PORT:-0}"
if ! [[ "$port" =~ ^[0-9]+$ ]] || (( port != 0 && (port < 1024 || port > 65535) )); then
	echo "错误：PITR_PORT 必须为 0（自动分配）或 1024-65535 的端口" >&2
  exit 2
fi
project="velora-pitr-$RANDOM-$$"
archive_dir="$(mktemp -d -t velora-pitr-archive.XXXXXX)"
base_dir="$(mktemp -d -t velora-pitr-base.XXXXXX)"
restore_socket="$(mktemp -d -t velora-pitr-socket.XXXXXX)"
backup_dir="$(mktemp -d -t velora-dual-backup.XXXXXX)"
restore_port="${PITR_RESTORE_PORT:-}"
restore_container="${project}-restore"
restore_volume="${project}-restore-data"
postgres_image="${DOCKER_REGISTRY:-docker.m.daocloud.io}/library/postgres:16-alpine"
restore_db_suffix="${RANDOM}_$$"
velora_restore="velora_restore_${restore_db_suffix}"
casdoor_restore="casdoor_restore_${restore_db_suffix}"
compose=(docker compose -p "$project" -f deployments/compose/local-pitr.yml)

cleanup() {
  docker rm -f "$restore_container" >/dev/null 2>&1 || true
  docker volume rm "$restore_volume" >/dev/null 2>&1 || true
  PGPASSWORD=postgres dropdb -h 127.0.0.1 -p "$port" -U postgres --if-exists "$velora_restore" >/dev/null 2>&1 || true
  PGPASSWORD=postgres dropdb -h 127.0.0.1 -p "$port" -U postgres --if-exists "$casdoor_restore" >/dev/null 2>&1 || true
  PITR_ARCHIVE_DIR="$archive_dir" PITR_PORT="$port" "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

PITR_ARCHIVE_DIR="$archive_dir" PITR_PORT="$port" "${compose[@]}" up -d
if [[ "$port" == 0 ]]; then
  port="$(PITR_ARCHIVE_DIR="$archive_dir" PITR_PORT="$port" "${compose[@]}" port pitr-postgres 5432 | sed -E 's/.*:([0-9]+)$/\1/')"
fi
if ! [[ "$port" =~ ^[0-9]+$ ]] || (( port < 1024 || port > 65535 )); then
  echo "错误：无法解析 PITR 容器映射端口" >&2
  exit 1
fi
if [[ -z "$restore_port" ]]; then restore_port=$((port + 1)); fi
if ! [[ "$restore_port" =~ ^[0-9]+$ ]] || (( restore_port < 1024 || restore_port > 65535 )); then
  echo "错误：无法分配 PITR 恢复端口" >&2
  exit 1
fi
for _ in $(seq 1 60); do
  if pg_isready -h 127.0.0.1 -p "$port" -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
pg_isready -h 127.0.0.1 -p "$port" -U postgres >/dev/null

db_url="postgres://postgres:postgres@127.0.0.1:${port}/velora?sslmode=disable"
idp_url="postgres://postgres:postgres@127.0.0.1:${port}/casdoor?sslmode=disable"
marker="${project}-marker"
PGPASSWORD=postgres psql "$db_url" -v ON_ERROR_STOP=1 -c "CREATE TABLE pitr_contract_markers(marker text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now()); INSERT INTO pitr_contract_markers(marker) VALUES ('$marker-before');"
PGPASSWORD=postgres psql "$idp_url" -v ON_ERROR_STOP=1 -c "CREATE TABLE pitr_contract_markers(marker text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now()); INSERT INTO pitr_contract_markers(marker) VALUES ('$marker-before');"
before_restore_point="$(PGPASSWORD=postgres psql "$db_url" -Atc "SELECT pg_create_restore_point('${project}-before-marker');" | tr -d '[:space:]')"

PITR_WAL_ARCHIVE_URI="file://${archive_dir}" DATABASE_URL="$db_url" PITR_REQUIRE_PRIMARY=true ./scripts/check-pitr-config.sh
container_id="$(PITR_ARCHIVE_DIR="$archive_dir" PITR_PORT="$port" "${compose[@]}" ps -q pitr-postgres)"
PITR_ARCHIVE_DIR="$archive_dir" PITR_PORT="$port" "${compose[@]}" exec -T pitr-postgres rm -rf /tmp/pitr-base
PITR_ARCHIVE_DIR="$archive_dir" PITR_PORT="$port" "${compose[@]}" exec -T pitr-postgres pg_basebackup -U postgres -D /tmp/pitr-base -Fp -X stream -c fast >/dev/null
docker cp "$container_id:/tmp/pitr-base/." "$base_dir"

PGPASSWORD=postgres psql "$db_url" -v ON_ERROR_STOP=1 -c "INSERT INTO pitr_contract_markers(marker) VALUES ('$marker-after');"
PGPASSWORD=postgres psql "$idp_url" -v ON_ERROR_STOP=1 -c "INSERT INTO pitr_contract_markers(marker) VALUES ('$marker-after');"
PGPASSWORD=postgres psql "$db_url" -v ON_ERROR_STOP=1 -c "SELECT pg_create_restore_point('${project}-after-marker'); SELECT pg_switch_wal();"
PGPASSWORD=postgres psql "$idp_url" -v ON_ERROR_STOP=1 -c "SELECT pg_switch_wal();"
sleep 8
archive_count="$(find "$archive_dir" -type f | wc -l | tr -d '[:space:]')"
if [[ "$archive_count" -eq 0 ]]; then
  echo "错误：未发现 WAL 归档文件，PITR 证据不成立" >&2
  exit 1
fi

printf "restore_command = 'cp /wal_archive/%%f %%p'\nrecovery_target_name = '%s-after-marker'\nrecovery_target_action = 'promote'\n" "$project" >> "$base_dir/postgresql.auto.conf"
touch "$base_dir/recovery.signal"
chmod 700 "$base_dir"
restore_volume_id="$(docker volume create "$restore_volume")"
docker run --rm --entrypoint bash -v "$base_dir:/from:ro" -v "$restore_volume_id:/to" "$postgres_image" -ec 'cp -a /from/. /to/ && chown -R postgres:postgres /to'
docker run -d --name "$restore_container" -p "127.0.0.1:${restore_port}:5432" -v "$restore_volume_id:/var/lib/postgresql/data" -v "$archive_dir:/wal_archive" "$postgres_image" postgres >/dev/null
for _ in $(seq 1 60); do
  if pg_isready -h 127.0.0.1 -p "$restore_port" -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
pg_isready -h 127.0.0.1 -p "$restore_port" -U postgres >/dev/null
restored_before="$(PGPASSWORD=postgres psql "postgres://postgres:postgres@127.0.0.1:${restore_port}/velora?sslmode=disable" -Atc "SELECT count(*) FROM pitr_contract_markers WHERE marker='${marker}-before';" | tr -d '[:space:]')"
restored_after="$(PGPASSWORD=postgres psql "postgres://postgres:postgres@127.0.0.1:${restore_port}/velora?sslmode=disable" -Atc "SELECT count(*) FROM pitr_contract_markers WHERE marker='${marker}-after';" | tr -d '[:space:]')"
if [[ "$restored_before" != 1 || "$restored_after" != 1 ]]; then
  echo "错误：PITR 恢复后 marker 校验失败 before=$restored_before after=$restored_after" >&2
  exit 1
fi
docker rm -f "$restore_container" >/dev/null

backup_stamp="$(date +%Y%m%d_%H%M%S)"
DATABASE_URL="$db_url" CASDOOR_DATABASE_URL="$idp_url" BACKUP_DIR="$backup_dir" BACKUP_STAMP="$backup_stamp" BACKUP_RETENTION_DAYS=1 BACKUP_USE_CONTAINER=true BACKUP_DATABASE_HOST=127.0.0.1 POSTGRES_CONTAINER="$container_id" ./scripts/backup-all-databases.sh >/dev/null
velora_dump="$backup_dir/velora_full_${backup_stamp}.dump"
casdoor_dump="$backup_dir/casdoor_full_${backup_stamp}.dump"
[[ -s "$velora_dump" && -s "$casdoor_dump" ]] || { echo "错误：双库备份文件缺失" >&2; exit 1; }
sha256sum -c "$velora_dump.sha256" >/dev/null 2>&1 || shasum -a 256 -c "$velora_dump.sha256" >/dev/null
sha256sum -c "$casdoor_dump.sha256" >/dev/null 2>&1 || shasum -a 256 -c "$casdoor_dump.sha256" >/dev/null

PGPASSWORD=postgres psql "postgres://postgres:postgres@127.0.0.1:${port}/postgres?sslmode=disable" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${velora_restore};" >/dev/null
PGPASSWORD=postgres psql "postgres://postgres:postgres@127.0.0.1:${port}/postgres?sslmode=disable" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${casdoor_restore};" >/dev/null
docker cp "$velora_dump" "$container_id:/tmp/velora-restore.dump"
docker cp "$casdoor_dump" "$container_id:/tmp/casdoor-restore.dump"
docker exec "$container_id" pg_restore -h 127.0.0.1 -U postgres -d "$velora_restore" --no-owner --no-acl /tmp/velora-restore.dump >/dev/null
docker exec "$container_id" pg_restore -h 127.0.0.1 -U postgres -d "$casdoor_restore" --no-owner --no-acl /tmp/casdoor-restore.dump >/dev/null
dual_before="$(PGPASSWORD=postgres psql "postgres://postgres:postgres@127.0.0.1:${port}/${velora_restore}?sslmode=disable" -Atc "SELECT count(*) FROM pitr_contract_markers WHERE marker='${marker}-before';" | tr -d '[:space:]')"
dual_idp_before="$(PGPASSWORD=postgres psql "postgres://postgres:postgres@127.0.0.1:${port}/${casdoor_restore}?sslmode=disable" -Atc "SELECT count(*) FROM pitr_contract_markers WHERE marker='${marker}-before';" | tr -d '[:space:]')"
[[ "$dual_before" == 1 && "$dual_idp_before" == 1 ]] || { echo "错误：双库恢复 marker 校验失败" >&2; exit 1; }

evidence_dir="${VELORA_ACCEPTANCE_EVIDENCE_DIR:-./artifacts/acceptance}"
mkdir -p "$evidence_dir"
evidence="$evidence_dir/dual-db-pitr-${backup_stamp}.json"
python3 - "$evidence" "$before_restore_point" "$archive_count" "$velora_dump" "$casdoor_dump" <<'PY'
import json, sys
path, restore_point, archive_count, velora_dump, casdoor_dump = sys.argv[1:]
with open(path, "w", encoding="utf-8") as stream:
    json.dump({"schema": "velora.acceptance.dual-db-pitr.v1", "status": "passed", "restore_point_before_marker": restore_point, "wal_archive_files": int(archive_count), "dual_backup_files": [velora_dump, casdoor_dump], "notes": "local contract only; production RPO/RTO and cross-region restore still require scheduled drills"}, stream, ensure_ascii=False, indent=2)
    stream.write("\n")
PY
echo "Dual database backup + WAL/PITR smoke passed; evidence=$evidence"
