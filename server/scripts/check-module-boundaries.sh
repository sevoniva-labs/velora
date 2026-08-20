#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

forbidden='github\.com/go-kratos|google\.golang\.org/grpc|api/gen|internal/platform|database/sql|franz-go|rocketmq|aws-sdk-go|nacos-sdk'

if matches="$(rg -n "$forbidden" examples/settlement/domain examples/settlement/application)"; then
  echo "module boundary violation: domain/application imports transport or infrastructure" >&2
  printf '%s\n' "$matches" >&2
  exit 1
fi

for required in \
  examples/settlement/domain/settlement.go \
  examples/settlement/application/query.go \
  examples/settlement/transport/service.go \
  cmd/example-settlement-service/main.go \
  api/proto/forge/v1/reference_settlement.proto; do
  if [[ ! -f "$required" ]]; then
    echo "reference split-service component is missing: $required" >&2
    exit 1
  fi
done

echo "module boundaries OK: settlement domain/application remain infrastructure-independent"
