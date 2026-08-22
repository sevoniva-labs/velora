#!/usr/bin/env bash
# Lightweight production synthetic monitor for low-resource deployments.
set -euo pipefail
umask 077

RUNTIME_DIR="${VELORA_RUNTIME_DIR:-/opt/velora/prod/runtime}"
STATUS_FILE="${VELORA_HEALTH_STATUS_FILE:-$RUNTIME_DIR/evidence/health-last.json}"
BACKUP_STATUS="$RUNTIME_DIR/evidence/backup-last-success.json"
MAX_BACKUP_AGE_SECONDS="${VELORA_MAX_BACKUP_AGE_SECONDS:-129600}"
CERT_WARN_DAYS="${VELORA_CERT_WARN_DAYS:-30}"
failures=()

check_url() {
  local name="$1" url="$2" expected="${3:-}"
  local body
  if ! body="$(curl --fail --silent --show-error --max-time 10 --connect-timeout 3 "$url")"; then
    failures+=("$name unavailable")
    return
  fi
  if [[ -n "$expected" && "$body" != *"$expected"* ]]; then
    failures+=("$name returned unexpected content")
  fi
}

check_url "portal readiness" "https://home.sevoniva.com/healthz"
check_url "OIDC discovery" "https://auth.sevoniva.com/.well-known/openid-configuration" '"issuer":"https://auth.sevoniva.com"'
check_url "Spectra readiness" "https://spectra.sevoniva.com/api/v1/system/health"

for container in velora-prod-postgres velora-prod-redis velora-prod-casdoor velora-prod-server velora-prod-worker velora-prod-web velora-prod-edge; do
  state="$(docker inspect --format '{{.State.Status}}' "$container" 2>/dev/null || true)"
  health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)"
  if [[ "$state" != "running" || ("$health" != "healthy" && "$health" != "none") ]]; then
    failures+=("container $container state=$state health=$health")
  fi
done

now_epoch="$(date +%s)"
if [[ ! -s "$BACKUP_STATUS" ]]; then
  failures+=("backup success evidence missing")
else
  backup_time="$(jq -r '.completed_at // empty' "$BACKUP_STATUS")"
  backup_epoch="$(date -d "$backup_time" +%s 2>/dev/null || printf 0)"
  if (( backup_epoch == 0 || now_epoch - backup_epoch > MAX_BACKUP_AGE_SECONDS )); then
    failures+=("latest verified backup is stale")
  fi
fi

for cert in "$RUNTIME_DIR"/certs/*/fullchain.pem; do
  [[ -r "$cert" ]] || { failures+=("TLS certificate missing"); continue; }
  if ! openssl x509 -checkend "$((CERT_WARN_DAYS * 86400))" -noout -in "$cert" >/dev/null; then
    failures+=("certificate expires within ${CERT_WARN_DAYS} days: $(basename "$(dirname "$cert")")")
  fi
done

mkdir -p "$(dirname "$STATUS_FILE")"
tmp_status="${STATUS_FILE}.tmp"
status="passed"
if ((${#failures[@]} > 0)); then status="failed"; fi
jq -n \
  --arg status "$status" \
  --arg checked_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson failures "$(printf '%s\n' "${failures[@]:-}" | jq -Rsc 'split("\n") | map(select(length > 0))')" \
  '{schema:"velora.health.status.v1",status:$status,checked_at:$checked_at,failures:$failures}' >"$tmp_status"
chmod 600 "$tmp_status"
mv -f "$tmp_status" "$STATUS_FILE"

if ((${#failures[@]} > 0)); then
  printf 'production health check failed: %s\n' "$(IFS='; '; echo "${failures[*]}")" >&2
  webhook_file="$RUNTIME_DIR/secrets/ops-alert-webhook"
  if [[ -r "$webhook_file" && -s "$webhook_file" ]]; then
    curl --fail --silent --show-error --max-time 10 \
      -H 'Content-Type: application/json' \
      --data-binary "@$STATUS_FILE" "$(<"$webhook_file")" >/dev/null || true
  fi
  exit 1
fi
echo "production health check passed"
