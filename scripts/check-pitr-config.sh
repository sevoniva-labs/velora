#!/usr/bin/env bash
# Read-only PostgreSQL PITR prerequisite check.
#
# This command never changes PostgreSQL configuration or archives WAL. It
# verifies that the target primary exposes the settings required for continuous
# WAL archiving and that an approved object-storage target has been supplied.
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

DATABASE_URL="${DATABASE_URL:-}"
DATABASE_URL_FILE="${DATABASE_URL_FILE:-${VELORA_DATABASE_DSN_FILE:-}}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-velora-postgres}"
PITR_WAL_ARCHIVE_URI="${PITR_WAL_ARCHIVE_URI:-}"
PITR_REQUIRE_PRIMARY="${PITR_REQUIRE_PRIMARY:-true}"

if [[ -n "$DATABASE_URL_FILE" ]]; then
  if [[ ! -r "$DATABASE_URL_FILE" ]]; then
    echo "错误：DATABASE_URL_FILE 不可读" >&2
    exit 1
  fi
  IFS= read -r DATABASE_URL < "$DATABASE_URL_FILE"
fi
if [[ -z "$DATABASE_URL" ]]; then
  echo "错误：DATABASE_URL 或 DATABASE_URL_FILE 未设置" >&2
  exit 1
fi
if [[ -z "$PITR_WAL_ARCHIVE_URI" || ! "$PITR_WAL_ARCHIVE_URI" =~ ^[a-zA-Z][a-zA-Z0-9+.-]*://[^[:space:]]+$ ]]; then
  echo "错误：PITR_WAL_ARCHIVE_URI 必须是无空白的对象存储 URI" >&2
  exit 1
fi
if [[ "$PITR_REQUIRE_PRIMARY" != true && "$PITR_REQUIRE_PRIMARY" != false ]]; then
  echo "错误：PITR_REQUIRE_PRIMARY 必须为 true 或 false" >&2
  exit 1
fi

if command -v psql >/dev/null 2>&1; then
  PSQL=(psql)
else
  if ! command -v docker >/dev/null 2>&1; then
    echo "错误：本机无 psql 且 docker 不可用" >&2
    exit 1
  fi
  if ! [[ "$POSTGRES_CONTAINER" =~ ^[a-zA-Z0-9_.-]+$ ]]; then
    echo "错误：POSTGRES_CONTAINER 含非法字符" >&2
    exit 1
  fi
  PSQL=(docker exec "$POSTGRES_CONTAINER" psql)
  DATABASE_URL="$(echo "$DATABASE_URL" | sed -E 's#(postgres://[^@/]+@)[^:/]+(:[0-9]+)?/#\1postgres:5432/#')"
fi

SETTINGS_FILE="$(mktemp -t velora-pitr-settings.XXXXXX)"
trap 'rm -f "$SETTINGS_FILE"' EXIT
"${PSQL[@]}" "$DATABASE_URL" -v ON_ERROR_STOP=1 -At -F $'\t' -c \
  "SELECT current_setting('wal_level'), current_setting('archive_mode'), current_setting('archive_command'), current_setting('archive_timeout'), pg_is_in_recovery();" \
  > "$SETTINGS_FILE"

IFS=$'\t' read -r WAL_LEVEL ARCHIVE_MODE ARCHIVE_COMMAND ARCHIVE_TIMEOUT IN_RECOVERY < "$SETTINGS_FILE"
WAL_LEVEL="${WAL_LEVEL,,}"
ARCHIVE_MODE="${ARCHIVE_MODE,,}"
IN_RECOVERY="${IN_RECOVERY,,}"

if [[ "$WAL_LEVEL" != replica && "$WAL_LEVEL" != logical ]]; then
  echo "错误：wal_level=$WAL_LEVEL；PITR 至少需要 replica" >&2
  exit 1
fi
if [[ "$ARCHIVE_MODE" != on && "$ARCHIVE_MODE" != always ]]; then
  echo "错误：archive_mode=$ARCHIVE_MODE；必须启用 WAL 归档" >&2
  exit 1
fi
if [[ -z "$ARCHIVE_COMMAND" || "$ARCHIVE_COMMAND" == "(disabled)" || "$ARCHIVE_COMMAND" != *%p* || "$ARCHIVE_COMMAND" != *%f* ]]; then
  echo "错误：archive_command 未配置有效的 %p/%f 归档命令" >&2
  exit 1
fi
if ! [[ "$ARCHIVE_TIMEOUT" =~ ^[0-9]+$ ]] || (( ARCHIVE_TIMEOUT <= 0 )); then
  echo "错误：archive_timeout 必须为大于 0 的秒数" >&2
  exit 1
fi
if [[ "$PITR_REQUIRE_PRIMARY" == true && "$IN_RECOVERY" != false ]]; then
  echo "错误：目标不是 primary（pg_is_in_recovery=$IN_RECOVERY）" >&2
  exit 1
fi

echo "PITR config check passed"
echo "wal_level=$WAL_LEVEL archive_mode=$ARCHIVE_MODE archive_timeout=${ARCHIVE_TIMEOUT}s primary=$([[ "$IN_RECOVERY" == false ]] && echo true || echo false)"
echo "wal_archive_target=configured"
