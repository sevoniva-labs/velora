#!/usr/bin/env bash
# ============================================================================
# Velora 数据库恢复脚本（Phase A2）— 恢复演练 / 故障恢复
#
# 用法：
#   ./scripts/restore-db.sh backups/velora_full_20260101_120000.dump
#   RESTORE_DB_URL=postgres://... ./scripts/restore-db.sh <备份文件>
#
# 默认恢复到 .env 的 DATABASE_URL；如需恢复到新库，用 RESTORE_DB_URL 覆盖。
# 恢复前会备份当前库（保险），并清空目标库（警告确认）。
# ============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

RESTORE_DB_URL="${RESTORE_DB_URL:-}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-}"
RESTORE_SAFETY_BACKUP_DIR="${RESTORE_SAFETY_BACKUP_DIR:-./backups/restore-safety}"
if [ -z "$RESTORE_DB_URL" ] && [ -f .env ]; then
  RESTORE_DB_URL="$(grep -E '^DATABASE_URL=' .env | head -1 | cut -d= -f2-)"
fi
if [ -z "$POSTGRES_CONTAINER" ] && [ -f .env ]; then
  POSTGRES_CONTAINER="$(grep -E '^POSTGRES_CONTAINER=' .env | head -1 | cut -d= -f2-)"
fi
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-postgres}"

if [ $# -lt 1 ]; then
  echo "用法：$0 <备份文件.dump> [RESTORE_DB_URL=...]" >&2
  exit 1
fi
DUMP_FILE="$1"
if [ ! -f "$DUMP_FILE" ]; then
  echo "错误：备份文件不存在：$DUMP_FILE" >&2
  exit 1
fi
if [ -z "$RESTORE_DB_URL" ]; then
  echo "错误：RESTORE_DB_URL 未设置（恢复目标库）" >&2
  exit 1
fi

if [[ "$DUMP_FILE" == *.age ]]; then
  if ! command -v age >/dev/null 2>&1 || [ -z "${BACKUP_AGE_IDENTITY_FILE:-}" ]; then
    echo "错误：加密备份恢复需要 age 和 BACKUP_AGE_IDENTITY_FILE" >&2
    exit 1
  fi
  DECRYPTED_FILE="$(mktemp "${TMPDIR:-/tmp}/velora-restore.XXXXXX.dump")"
  trap 'rm -f "$DECRYPTED_FILE"' EXIT
  age -d -i "$BACKUP_AGE_IDENTITY_FILE" -o "$DECRYPTED_FILE" "$DUMP_FILE"
  DUMP_FILE="$DECRYPTED_FILE"
fi

MANIFEST="${1}.sha256"
if [ -f "$MANIFEST" ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c "$MANIFEST"
  else
    shasum -a 256 -c "$MANIFEST"
  fi
fi

# 解析目标库名（用于预清理判断）
DB_NAME="$(echo "$RESTORE_DB_URL" | sed -E 's#postgres://[^/]+/([^?]+).*#\1#')"
if ! [[ "$DB_NAME" =~ ^[a-zA-Z0-9_]+$ ]]; then
  echo "错误：无法安全解析目标数据库名" >&2
  exit 1
fi

echo "==> 恢复前备份当前库（保险）…"
mkdir -p "$RESTORE_SAFETY_BACKUP_DIR"
DATABASE_URL="$RESTORE_DB_URL" BACKUP_DIR="$RESTORE_SAFETY_BACKUP_DIR" ./scripts/backup-db.sh

echo "==> 目标库：$DB_NAME"
echo "警告：恢复将清空目标库现有数据。输入 yes 继续："
CONFIRM="${RESTORE_CONFIRM:-}"
if [ -z "$CONFIRM" ]; then
  read -r CONFIRM
fi
if [ "$CONFIRM" != "yes" ]; then
  echo "已取消。"
  exit 0
fi

if command -v pg_restore >/dev/null 2>&1; then
  PG_RESTORE=(pg_restore)
elif [ -x /opt/homebrew/bin/pg_restore ]; then
  PG_RESTORE=(/opt/homebrew/bin/pg_restore)
else
  echo "==> 本机无 pg_restore，尝试通过 docker compose 容器执行…"
  if ! [[ "$POSTGRES_CONTAINER" =~ ^[a-zA-Z0-9_.-]+$ ]]; then
    echo "错误：POSTGRES_CONTAINER 含非法字符" >&2
    exit 1
  fi
  PG_RESTORE=(docker exec "$POSTGRES_CONTAINER" pg_restore)
  # 容器内连接串：host 换 compose 服务名，端口统一 5432
  RESTORE_DB_URL="$(echo "$RESTORE_DB_URL" | sed -E 's#(postgres://[^@/]+@)[^:/]+(:[0-9]+)?/#\1postgres:5432/#')"
fi

echo "==> 开始恢复：$DUMP_FILE"
"${PG_RESTORE[@]}" "$RESTORE_DB_URL" --clean --if-exists --no-owner -v "$DUMP_FILE"

echo "==> 恢复完成。建议执行：docker compose restart server（或 make dev-server）"
echo "==> 验证：curl http://localhost:8080/healthz"
