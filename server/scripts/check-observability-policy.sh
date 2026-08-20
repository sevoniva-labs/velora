#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

collector=deploy/observability/otel-collector-production.yaml
prometheus=deploy/observability/prometheus-production.yml

required_collector=(
  'client_ca_file: ${env:VELORA_OTEL_RECEIVER_CLIENT_CA_FILE}'
  'ca_file: ${env:VELORA_OTEL_EXPORT_CA_FILE}'
  'cert_file: ${env:VELORA_OTEL_EXPORT_CERT_FILE}'
  'key_file: ${env:VELORA_OTEL_EXPORT_KEY_FILE}'
  'storage: file_storage'
  'retry_on_failure:'
  'insecure: false'
)
for value in "${required_collector[@]}"; do
  rg -Fq "$value" "$collector" || {
    echo "production Collector policy missing: $value" >&2
    exit 1
  }
done
if rg -n '^\s*debug:|insecure:\s*true' "$collector"; then
  echo "production Collector must not use debug exporter or insecure TLS" >&2
  exit 1
fi

for value in 'scheme: https' 'ca_file:' 'cert_file:' 'key_file:' 'insecure_skip_verify: false'; do
  rg -Fq "$value" "$prometheus" || {
    echo "production Prometheus policy missing: $value" >&2
    exit 1
  }
done

if [[ -n "${OTELCOL_BIN:-}" ]]; then
  "$OTELCOL_BIN" validate --config "$collector"
  echo "observability runtime config validated with $OTELCOL_BIN"
else
  echo "observability static policy OK; runtime Collector validation not executed (OTELCOL_BIN unset)"
fi
