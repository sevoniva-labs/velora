#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d -t velora-pitr-test.XXXXXX)"
trap 'rm -rf "$TMP"' EXIT

cat > "$TMP/psql" <<'EOF'
#!/usr/bin/env bash
case "${PITR_FIXTURE:-ok}" in
ok) printf 'replica\ton\tarchive-wal --path=%%p --name=%%f\t60\tfalse\n' ;;
bad-command) printf 'replica\ton\t(disabled)\t60\tfalse\n' ;;
bad-replica) printf 'replica\ton\tarchive-wal --path=%%p --name=%%f\t60\ttrue\n' ;;
bad-timeout) printf 'replica\ton\tarchive-wal --path=%%p --name=%%f\t0\tfalse\n' ;;
*) exit 2 ;;
esac
EOF
chmod 700 "$TMP/psql"

PATH="$TMP:$PATH" DATABASE_URL='postgres://test@localhost/velora' PITR_WAL_ARCHIVE_URI='s3://approved/wal' \
  bash "$ROOT/scripts/check-pitr-config.sh" >/dev/null

if PATH="$TMP:$PATH" PITR_FIXTURE=bad-command DATABASE_URL='postgres://test@localhost/velora' PITR_WAL_ARCHIVE_URI='s3://approved/wal' \
  bash "$ROOT/scripts/check-pitr-config.sh" >/dev/null 2>&1; then
  echo "expected invalid archive_command to fail" >&2
  exit 1
fi
if PATH="$TMP:$PATH" PITR_FIXTURE=bad-replica DATABASE_URL='postgres://test@localhost/velora' PITR_WAL_ARCHIVE_URI='s3://approved/wal' \
  bash "$ROOT/scripts/check-pitr-config.sh" >/dev/null 2>&1; then
  echo "expected standby target to fail" >&2
  exit 1
fi
if PATH="$TMP:$PATH" PITR_FIXTURE=bad-timeout DATABASE_URL='postgres://test@localhost/velora' PITR_WAL_ARCHIVE_URI='s3://approved/wal' \
  bash "$ROOT/scripts/check-pitr-config.sh" >/dev/null 2>&1; then
  echo "expected zero archive_timeout to fail" >&2
  exit 1
fi

echo "check-pitr-config tests passed"
