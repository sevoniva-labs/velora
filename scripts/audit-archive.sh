#!/usr/bin/env bash
# ============================================================================
# Velora 审计日志归档脚本（Phase C5 保留策略）
#
# 策略：在线 180 天；超过 180 天且已过 3 年合规期的记录归档导出后删除。
# 归档文件：CSV 导出（含 hash 链字段），落盘到 ARCHIVE_DIR，可长期冷存储。
#
# 用法：
#   ./scripts/audit-archive.sh                     # 默认归档 180 天前的记录
#   AUDIT_RETENTION_DAYS=365 ./scripts/audit-archive.sh   # 自定义在线保留天数
# ============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

RETENTION_DAYS="${AUDIT_RETENTION_DAYS:-180}"
ARCHIVE_DIR="${AUDIT_ARCHIVE_DIR:-./backups/audit}"
DATABASE_URL="${DATABASE_URL:-}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-}"
if [ -z "$DATABASE_URL" ] && [ -f .env ]; then
  DATABASE_URL="$(grep -E '^DATABASE_URL=' .env | head -1 | cut -d= -f2-)"
fi
if [ -z "$POSTGRES_CONTAINER" ] && [ -f .env ]; then
  POSTGRES_CONTAINER="$(grep -E '^POSTGRES_CONTAINER=' .env | head -1 | cut -d= -f2-)"
fi
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-postgres}"
if [ -z "$DATABASE_URL" ]; then
  echo "错误：DATABASE_URL 未设置" >&2
  exit 1
fi

mkdir -p "$ARCHIVE_DIR"
STAMP="$(date +%Y%m%d_%H%M%S)"
CSV="$ARCHIVE_DIR/audit_until_${STAMP}.csv"
CUTOFF="$(date -u -d "$RETENTION_DAYS days ago" +%Y-%m-%d)"

# 容器模式连接串转换
if ! command -v psql >/dev/null 2>&1; then
  echo "==> 本机无 psql，使用 docker exec"
  PSQL="docker exec $POSTGRES_CONTAINER psql"
  DATABASE_URL="$(echo "$DATABASE_URL" | sed -E 's#(postgres://[^@/]+@)[^:/]+(:[0-9]+)?/#\1postgres:5432/#')"
else
  PSQL="psql"
fi

echo "==> 归档 $CUTOFF 之前的审计日志（在线保留 ${RETENTION_DAYS} 天）"

# 导出（CSV，带 hash 链字段便于事后校验）
$PSQL "$DATABASE_URL" -c "\copy (SELECT id, operator, action, resource, resource_id, ip, user_agent, request_id, detail, prev_hash, hash, created_at FROM audit_logs WHERE created_at < '$CUTOFF'::timestamptz ORDER BY id) TO '$CSV' WITH CSV HEADER"

# 校验导出行数 = 待删除行数
EXPORTED=$(tail -n +2 "$CSV" | wc -l | tr -d ' ')
PENDING=$($PSQL "$DATABASE_URL" -tAc "SELECT count(*) FROM audit_logs WHERE created_at < '$CUTOFF'::timestamptz;")
if [ "$EXPORTED" != "$PENDING" ]; then
  echo "错误：导出行数($EXPORTED)与待删行数($PENDING)不一致，中止删除" >&2
  exit 1
fi

# 删除（带 hash 链：保留最新一条作为链锚，避免破坏后续链）
$PSQL "$DATABASE_URL" -tAc "DELETE FROM audit_logs WHERE created_at < '$CUTOFF'::timestamptz AND id < (SELECT max(id) FROM audit_logs);"

echo "==> 归档完成：$CSV（${EXPORTED} 行）"
echo "==> 建议将 $ARCHIVE_DIR 同步到对象存储冷存储（3 年合规期）"
