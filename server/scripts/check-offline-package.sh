#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

required=(
  go.mod
  go.sum
  tools/go.mod
  tools/go.sum
  pnpm-lock.yaml
  configs/xinchuang.yaml
  deploy/helm/forge/values-xinchuang.yaml
)
for path in "${required[@]}"; do
  [[ -s "$path" ]] || { echo "offline prerequisite missing: $path" >&2; exit 1; }
done

if rg -n -I 'docker\.io|ghcr\.io|quay\.io|registry-1\.docker\.io' configs deploy tools package.json pnpm-lock.yaml; then
  echo "public OCI source found in offline prerequisites" >&2
  exit 1
fi

bundle=${OFFLINE_BUNDLE_DIR:-}
if [[ -z "$bundle" ]]; then
  echo "offline repository prerequisites OK; OFFLINE_BUNDLE_DIR not supplied, bundle evidence not claimed"
  exit 0
fi
[[ -d "$bundle" ]] || { echo "OFFLINE_BUNDLE_DIR is not a directory: $bundle" >&2; exit 1; }
[[ -s "$bundle/manifest.sha256" ]] || { echo "offline bundle manifest.sha256 is required" >&2; exit 1; }
[[ -s "$bundle/provenance.txt" ]] || { echo "offline bundle provenance.txt is required" >&2; exit 1; }
[[ -s "$bundle/images.lock" ]] || { echo "offline bundle images.lock is required" >&2; exit 1; }
if rg -n -I 'docker\.io|ghcr\.io|quay\.io|registry-1\.docker\.io' "$bundle"; then
  echo "public OCI source found in offline bundle" >&2
  exit 1
fi
if awk 'NF && $0 !~ /@sha256:[0-9a-f]{64}/ {bad=1} END {exit bad}' "$bundle/images.lock"; then
  :
else
  echo "every offline image must be pinned by sha256 digest" >&2
  exit 1
fi
(cd "$bundle" && shasum -a 256 -c manifest.sha256)
echo "offline bundle evidence verified: $bundle"
