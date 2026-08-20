#!/usr/bin/env bash
# 以同一时间戳备份 Velora 与 Casdoor 数据库。
# 两个库必须来自同一个 PostgreSQL 实例/恢复策略；任一备份失败即返回非零。
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

DATABASE_URL="${DATABASE_URL:-}"
CASDOOR_DATABASE_URL="${CASDOOR_DATABASE_URL:-}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-postgres}"
BACKUP_STAMP="${BACKUP_STAMP:-$(date +%Y%m%d_%H%M%S)}"

if [ -z "$DATABASE_URL" ] || [ -z "$CASDOOR_DATABASE_URL" ]; then
  echo "错误：DATABASE_URL 与 CASDOOR_DATABASE_URL 均为必填，不能只备份单一身份数据库" >&2
  exit 1
fi
if ! [[ "$BACKUP_STAMP" =~ ^[0-9]{8}_[0-9]{6}$ ]]; then
  echo "错误：BACKUP_STAMP 必须为 YYYYMMDD_HHMMSS" >&2
  exit 1
fi

common_env=(
  "BACKUP_DIR=$BACKUP_DIR"
  "BACKUP_STAMP=$BACKUP_STAMP"
  "BACKUP_RETENTION_DAYS=${BACKUP_RETENTION_DAYS:-30}"
  "BACKUP_ENCRYPTION_KEY_FILE=${BACKUP_ENCRYPTION_KEY_FILE:-}"
  "BACKUP_ENCRYPTION_REQUIRED=${BACKUP_ENCRYPTION_REQUIRED:-false}"
  "BACKUP_SIGNING_KEY_FILE=${BACKUP_SIGNING_KEY_FILE:-}"
  "BACKUP_SIGNING_REQUIRED=${BACKUP_SIGNING_REQUIRED:-false}"
  "BACKUP_S3=${BACKUP_S3:-}"
  "POSTGRES_CONTAINER=$POSTGRES_CONTAINER"
)

echo "==> 使用恢复点时间戳：$BACKUP_STAMP"
env "${common_env[@]}" DATABASE_URL="$DATABASE_URL" BACKUP_FILENAME_PREFIX=velora_full ./scripts/backup-db.sh
env "${common_env[@]}" DATABASE_URL="$CASDOOR_DATABASE_URL" BACKUP_FILENAME_PREFIX=casdoor_full ./scripts/backup-db.sh
echo "==> Velora 与 Casdoor 双库备份完成"
