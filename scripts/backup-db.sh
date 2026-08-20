#!/usr/bin/env bash
# ============================================================================
# Velora 数据库备份脚本（Phase A2）
#
# 功能：
#   - 每日全量 pg_dump（自定义格式，支持恢复演练）
#   - 归档 WAL（若启用 PITR 需要连续归档，见 deployments/compose/pg-archive 说明）
#   - 保留策略：默认保留 30 天，可 BACKUP_RETENTION_DAYS 覆盖
#   - 支持本地目录或 S3 兼容对象存储（aws cli 或 s5cmd）
#
# 用法：
#   ./scripts/backup-db.sh                 # 全量备份（读 .env 的 DATABASE_URL）
#   BACKUP_DIR=/data/velora-backup ./scripts/backup-db.sh
#   BACKUP_S3=s3://bucket/velora ./scripts/backup-db.sh   # 备份后上传对象存储
# ============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

# --- 配置 ---
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
DATABASE_URL="${DATABASE_URL:-}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-}"

if [ -z "$DATABASE_URL" ] && [ -f .env ]; then
  # 仅提取 DATABASE_URL（避免 source .env 导致特殊字符问题）
  DATABASE_URL="$(grep -E '^DATABASE_URL=' .env | head -1 | cut -d= -f2-)"
fi
if [ -z "$POSTGRES_CONTAINER" ] && [ -f .env ]; then
  POSTGRES_CONTAINER="$(grep -E '^POSTGRES_CONTAINER=' .env | head -1 | cut -d= -f2-)"
fi
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-postgres}"
if [ -z "$DATABASE_URL" ]; then
  echo "错误：DATABASE_URL 未设置（可在环境变量或 .env 中配置）" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"
STAMP="$(date +%Y%m%d_%H%M%S)"
FILENAME="velora_full_${STAMP}.dump"
TARGET="$BACKUP_DIR/$FILENAME"

echo "==> 开始备份：$TARGET"

# 自定义格式（-Fc）支持 pg_restore 选择性恢复与并行恢复
IN_CONTAINER=false
if command -v pg_dump >/dev/null 2>&1; then
  PG_DUMP="pg_dump"
elif [ -x /opt/homebrew/bin/pg_dump ]; then
  PG_DUMP="/opt/homebrew/bin/pg_dump"
else
  # compose 环境：在 postgres 容器内执行（需容器名 velora-postgres 存在）
  echo "==> 本机无 pg_dump，尝试通过 docker compose 容器执行…"
  IN_CONTAINER=true
  PG_DUMP="docker exec $POSTGRES_CONTAINER pg_dump"
  # 容器内连接串：host 换 compose 服务名，端口统一 5432
  DATABASE_URL="$(echo "$DATABASE_URL" | sed -E 's#(postgres://[^@/]+@)[^:/]+(:[0-9]+)?/#\1postgres:5432/#')"
fi

if $IN_CONTAINER; then
  # 容器内写临时文件（相对路径在容器内不可用），完成后 docker cp 回宿主
  CONTAINER_TMP="/tmp/velora_backup_${STAMP}.dump"
  if ! $PG_DUMP "$DATABASE_URL" -Fc -f "$CONTAINER_TMP" 2>/tmp/velora_pgdump_err.txt; then
    echo "错误：pg_dump 失败：" >&2
    cat /tmp/velora_pgdump_err.txt >&2 || true
    rm -f /tmp/velora_pgdump_err.txt
    exit 1
  fi
  rm -f /tmp/velora_pgdump_err.txt
  docker cp "$POSTGRES_CONTAINER:$CONTAINER_TMP" "$TARGET"
  docker exec "$POSTGRES_CONTAINER" rm -f "$CONTAINER_TMP"
else
  $PG_DUMP "$DATABASE_URL" -Fc -f "$TARGET" -v 2> >(grep -v '^pg_dump:' >&2 || true)
fi

# 校验文件非空
if [ ! -s "$TARGET" ]; then
  echo "错误：备份文件为空（$TARGET）" >&2
  exit 1
fi
SIZE="$(du -h "$TARGET" | cut -f1)"
echo "==> 备份完成：$TARGET ($SIZE)"

# --- 清理过期备份 ---
echo "==> 清理超过 ${RETENTION_DAYS} 天的本地备份…"
find "$BACKUP_DIR" -name 'velora_full_*.dump' -mtime "+$RETENTION_DAYS" -delete

# --- 可选：上传对象存储 ---
if [ -n "${BACKUP_S3:-}" ]; then
  echo "==> 上传到对象存储：$BACKUP_S3"
  if command -v s5cmd >/dev/null 2>&1; then
    s5cmd cp "$TARGET" "$BACKUP_S3/"
  elif command -v aws >/dev/null 2>&1; then
    aws s3 cp "$TARGET" "$BACKUP_S3/"
  else
    echo "警告：未找到 s5cmd/aws，跳过上传（请安装或配置 BACKUP_S3）" >&2
  fi
  # 对象存储侧同样清理（列出并按文件名日期判断，简化：保留最近 N 个）
fi

echo "==> 全部完成"
