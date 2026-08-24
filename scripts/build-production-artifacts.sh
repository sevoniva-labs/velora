#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:?output directory is required}"
revision="${VELORA_REVISION:-$(git -C "$repo_root" rev-parse HEAD)}"
short_revision="$(printf '%s' "$revision" | cut -c1-12)"
release_version="${VELORA_VERSION:-0.2.0+$short_revision}"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$output_dir/server" "$output_dir/demo" "$output_dir/web/web-dist"
cd "$repo_root/server"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
for target in "velora:./cmd/server" "velora-worker:./cmd/worker" "velora-connect:./cmd/velora-connect" "velora-migrate:./cmd/migrate" "velora-storage-check:./cmd/storage-check"; do
  name="${target%%:*}"
  package="${target#*:}"
  ldflags="-s -w"
  if [[ "$name" != "velora-connect" ]]; then
    ldflags="$ldflags -X main.version=$release_version"
  fi
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$ldflags" -o "$output_dir/server/$name" "$package"
done
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$output_dir/demo/velora-oidc-demo" ./cmd/oidc-demo
cd "$repo_root/web"
export npm_config_registry="${npm_config_registry:-https://registry.npmmirror.com}"
VITE_APP_VERSION="$release_version" pnpm build
cp "$repo_root/deployments/docker/Dockerfile.server-artifact" "$output_dir/server/Dockerfile"
cp "$repo_root/deployments/docker/Dockerfile.demo-artifact" "$output_dir/demo/Dockerfile"
cp "$repo_root/deployments/docker/Dockerfile.web-artifact" "$output_dir/web/Dockerfile"
cp -R "$repo_root/web/dist/." "$output_dir/web/web-dist/"
(cd "$output_dir" && printf 'version=%s\nrevision=%s\nbuild_date=%s\n' "$release_version" "$revision" "$build_date" > BUILD-INFO)
(cd "$output_dir" && find server demo web -type f -print0 | sort -z | xargs -0 shasum -a 256 > SHA256SUMS)
echo "production artifacts: $output_dir ($release_version, $revision)"
