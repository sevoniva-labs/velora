#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-$repo_root/dist/velora-connect}"
mkdir -p "$output_dir"
cd "$repo_root/server"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os="${target%/*}"; arch="${target#*/}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="-s -w" -o "$output_dir/velora-connect_${os}_${arch}" ./cmd/velora-connect
done
cd "$output_dir"
shasum -a 256 velora-connect_* > SHA256SUMS
echo "velora-connect release artifacts: $output_dir"
