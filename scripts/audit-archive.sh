#!/usr/bin/env bash
# ============================================================================
# Velora 审计日志导出脚本（Phase C5 保留策略）
#
# 该脚本按真实 audit_logs schema 导出并生成校验清单，不直接删除在线数据。
# 生产删除必须走带 WORM receipt 与 audit_chain_anchors 的应用归档服务。
#
# 用法：
#   ./scripts/audit-archive.sh                     # 默认归档 180 天前的记录
#   AUDIT_RETENTION_DAYS=365 ./scripts/audit-archive.sh   # 自定义在线保留天数
# ============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

RETENTION_DAYS="${AUDIT_RETENTION_DAYS:-180}"
ARCHIVE_DIR="${AUDIT_ARCHIVE_DIR:-./backups/audit}"
DATABASE_URL="${DATABASE_URL:-}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-}"
if ! [[ "$RETENTION_DAYS" =~ ^[1-9][0-9]*$ ]]; then
  echo "错误：AUDIT_RETENTION_DAYS 必须是正整数" >&2
  exit 1
fi
if [[ -z "$ARCHIVE_DIR" || "$ARCHIVE_DIR" == "/" ]]; then
  echo "错误：AUDIT_ARCHIVE_DIR 不能为空或为根目录" >&2
  exit 1
fi
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
chmod 700 "$ARCHIVE_DIR"
STAMP="$(date +%Y%m%d_%H%M%S)"
CSV="$ARCHIVE_DIR/audit_until_${STAMP}.csv"
MANIFEST="$CSV.sha256"
METADATA="$CSV.metadata"
if date -u -d "$RETENTION_DAYS days ago" +%Y-%m-%dT%H:%M:%SZ >/tmp/velora-audit-cutoff.$$ 2>/dev/null; then
  CUTOFF="$(cat /tmp/velora-audit-cutoff.$$)"
  rm -f /tmp/velora-audit-cutoff.$$
else
  rm -f /tmp/velora-audit-cutoff.$$
  CUTOFF="$(date -u -v-"${RETENTION_DAYS}"d +%Y-%m-%dT%H:%M:%SZ)"
fi

# 容器模式连接串转换
if ! command -v psql >/dev/null 2>&1; then
  echo "==> 本机无 psql，使用 docker exec"
  if ! [[ "$POSTGRES_CONTAINER" =~ ^[a-zA-Z0-9_.-]+$ ]]; then
    echo "错误：POSTGRES_CONTAINER 含非法字符" >&2
    exit 1
  fi
  PSQL=(docker exec "$POSTGRES_CONTAINER" psql)
  DATABASE_URL="$(echo "$DATABASE_URL" | sed -E 's#(postgres://[^@/]+@)[^:/]+(:[0-9]+)?/#\1postgres:5432/#')"
else
  PSQL=(psql)
fi

echo "==> 归档 $CUTOFF 之前的审计日志（在线保留 ${RETENTION_DAYS} 天）"

# 导出真实 audit_logs schema（CSV，带 hash 链字段便于事后校验）
"${PSQL[@]}" "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "\copy (SELECT id, occurred_at, request_id, organization_id, actor_id, actor_name, action, resource_type, resource_id, result, client_ip, details_json, sequence_no, prev_hash, event_hash FROM audit_logs WHERE occurred_at < '${CUTOFF}'::timestamptz ORDER BY organization_id, sequence_no, id) TO STDOUT WITH CSV HEADER" > "$CSV"

# 校验导出行数 = 待归档行数；CSV 可能包含换行，不能用 wc 近似统计
if ! command -v python3 >/dev/null 2>&1; then
  echo "错误：需要 python3 解析 CSV，拒绝用 wc 近似行数" >&2
  exit 1
fi
EXPORTED=$(python3 - "$CSV" <<'PY'
import csv
import sys
with open(sys.argv[1], newline="", encoding="utf-8") as stream:
    print(sum(1 for _ in csv.DictReader(stream)))
PY
)
PENDING=$("${PSQL[@]}" "$DATABASE_URL" -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM audit_logs WHERE occurred_at < '${CUTOFF}'::timestamptz;" | tr -d '[:space:]')
if [ "$EXPORTED" != "$PENDING" ]; then
  echo "错误：导出行数($EXPORTED)与待删行数($PENDING)不一致，中止删除" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$CSV" > "$MANIFEST"
else
  shasum -a 256 "$CSV" > "$MANIFEST"
fi
{
  echo "cutoff=$CUTOFF"
  echo "event_count=$EXPORTED"
  echo "schema=audit_logs.v2"
  echo "delete=disabled"
} > "$METADATA"

echo "==> 归档完成：$CSV（${EXPORTED} 行）"
echo "==> 校验清单：$MANIFEST"
echo "==> 未删除在线数据：请由应用归档服务写入 WORM receipt/chain anchor 后再按审批流程清理"
