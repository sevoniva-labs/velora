#!/usr/bin/env bash
# 从同一时间戳备份恢复 Velora 与 Casdoor；目标库必须是隔离环境或经过变更审批。
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

if [ "$#" -ne 2 ]; then
  echo "用法：$0 <velora.dump[.age]> <casdoor.dump[.age]>" >&2
  exit 1
fi
VELORA_DUMP="$1"
CASDOOR_DUMP="$2"
RESTORE_DB_URL="${RESTORE_DB_URL:-}"
RESTORE_IDP_DB_URL="${RESTORE_IDP_DB_URL:-}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-postgres}"
RESTORE_CONFIRM="${RESTORE_CONFIRM:-}"

if [ -z "$RESTORE_DB_URL" ] || [ -z "$RESTORE_IDP_DB_URL" ]; then
  echo "错误：RESTORE_DB_URL 与 RESTORE_IDP_DB_URL 均为必填" >&2
  exit 1
fi
if [ ! -f "$VELORA_DUMP" ] || [ ! -f "$CASDOOR_DUMP" ]; then
  echo "错误：Velora/Casdoor 备份文件必须同时存在" >&2
  exit 1
fi
if [ "$RESTORE_CONFIRM" != "yes" ]; then
  echo "恢复将清空两个目标库；设置 RESTORE_CONFIRM=yes 后重试" >&2
  exit 1
fi

echo "==> 先恢复 Velora 数据库"
RESTORE_DB_URL="$RESTORE_DB_URL" POSTGRES_CONTAINER="$POSTGRES_CONTAINER" RESTORE_CONFIRM=yes \
  ./scripts/restore-db.sh "$VELORA_DUMP"
echo "==> 再恢复 Casdoor 数据库"
RESTORE_DB_URL="$RESTORE_IDP_DB_URL" POSTGRES_CONTAINER="$POSTGRES_CONTAINER" RESTORE_CONFIRM=yes \
  ./scripts/restore-db.sh "$CASDOOR_DUMP"
echo "==> Velora 与 Casdoor 双库恢复完成；请执行登录、权限和审计抽检"
