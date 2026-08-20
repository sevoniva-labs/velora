#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

TRIVY_VERSION=${TRIVY_VERSION:-0.74.0}
COSIGN_VERSION=${COSIGN_VERSION:-3.1.2}
OUT=${EVIDENCE_DIR:-.evidence}
: "${IMAGE_REF:?IMAGE_REF must be an internal OCI image pinned by sha256 digest}"
: "${TRIVY_DB_REPOSITORY:?TRIVY_DB_REPOSITORY must point to an internal Harbor mirror}"
: "${COSIGN_KEY:?COSIGN_KEY must point to the release signing key}"
: "${COSIGN_PUBLIC_KEY:?COSIGN_PUBLIC_KEY must point to the verification key}"

[[ "$IMAGE_REF" =~ ^[^/]+/.+@sha256:[0-9a-f]{64}$ ]] || {
  echo "IMAGE_REF must be registry/path@sha256:<64 hex>" >&2
  exit 1
}
for value in "$IMAGE_REF" "$TRIVY_DB_REPOSITORY"; do
  [[ "$value" != *docker.io* && "$value" != *ghcr.io* && "$value" != *quay.io* && "$value" != *aquasec* ]] || {
    echo "public OCI sources are forbidden: $value" >&2
    exit 1
  }
done

actual_trivy=$(trivy --version | awk 'NR==1 {print $2}')
actual_cosign=$(cosign version 2>/dev/null | awk '/GitVersion:/ {sub(/^v/, "", $2); print $2}')
[[ "$actual_trivy" == "$TRIVY_VERSION" ]] || { echo "trivy $TRIVY_VERSION required, got $actual_trivy" >&2; exit 1; }
[[ "$actual_cosign" == "$COSIGN_VERSION" ]] || { echo "cosign $COSIGN_VERSION required, got $actual_cosign" >&2; exit 1; }

mkdir -p "$OUT"
TRIVY_DB_REPOSITORY="$TRIVY_DB_REPOSITORY" trivy image \
  --skip-db-update --offline-scan --scanners vuln,secret \
  --severity HIGH,CRITICAL --exit-code 1 --format json \
  --output "$OUT/image-scan.json" "$IMAGE_REF"

cosign sign --yes --key "$COSIGN_KEY" "$IMAGE_REF"
cosign verify --key "$COSIGN_PUBLIC_KEY" --output-file "$OUT/cosign-verification.json" "$IMAGE_REF"
shasum -a 256 "$OUT/image-scan.json" "$OUT/cosign-verification.json" >"$OUT/IMAGE-SHA256SUMS"

echo "image scan and signature verification evidence generated in $OUT"
