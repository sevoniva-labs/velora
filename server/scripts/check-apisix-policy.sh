#!/usr/bin/env bash
set -euo pipefail

chart="deploy/helm/forge-apisix"
rendered="$(mktemp)"
trap 'rm -f "$rendered"' EXIT

common=(
  --set enabled=true
  --set hosts.web=forge.bank.internal
  --set hosts.grpc=grpc.forge.bank.internal
  --set governance.changeApprovalRef=CHG-20260817-001
)

python3 -m json.tool "$chart/values.schema.json" >/dev/null
helm lint "$chart" >/dev/null
helm lint "$chart" "${common[@]}" >/dev/null
helm template forge-apisix "$chart" "${common[@]}" >"$rendered"

for required in \
  'kind: ApisixPluginConfig' \
  'kind: ApisixRoute' \
  'kind: ApisixUpstream' \
  'kind: ApisixTls' \
  'velora.sevoniva.cn/change-approval-ref: "CHG-20260817-001"' \
  'allow_degradation: false' \
  'policy: "local"' \
  'retries: 0' \
  'scheme: https' \
  'scheme: grpcs'; do
  rg -q --fixed-strings "$required" "$rendered" || {
    echo "APISIX render missing required policy: $required" >&2
    exit 1
  }
done

expect_rejected() {
  local reason="$1"
  shift
  if helm template forge-apisix "$chart" "${common[@]}" "$@" >/dev/null 2>&1; then
    echo "APISIX production policy accepted: $reason" >&2
    exit 1
  fi
}

expect_rejected "missing change approval" --set governance.changeApprovalRef=
expect_rejected "placeholder hostname" --set hosts.web=velora.example.cn
expect_rejected "shared HTTP and gRPC hostname" --set hosts.grpc=forge.bank.internal
expect_rejected "plaintext upstream" --set upstream.tls=false
expect_rejected "gRPC without client mTLS" --set tls.grpcClientMTLS.enabled=false
expect_rejected "unreviewed controller version" --set compatibility.ingressControllerVersion=2.2.0

echo "APISIX policy OK: pinned compatibility and production fail-closed guards"
