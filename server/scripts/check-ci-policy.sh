#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

if [[ -d .github/workflows ]] && find .github/workflows -type f -print -quit | grep -q .; then
  echo "GitHub Actions workflows are forbidden; use GitLab CI or Jenkins" >&2
  exit 1
fi

files=(Makefile .gitlab-ci.yml Jenkinsfile)
patterns=(
  '@latest'
  'ubuntu-latest'
  'actions/'
  'github-actions'
  'GOPROXY=.*direct'
  'registry\.npmjs\.org'
)
for pattern in "${patterns[@]}"; do
  if rg -n -- "$pattern" "${files[@]}"; then
    echo "forbidden CI dependency pattern: $pattern" >&2
    exit 1
  fi
done

required_versions=(
  'github.com/anchore/syft v1.50.0'
	'github.com/bufbuild/buf v1.72.0'
	'github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2 v2.0.0-20260404020628-f149714c1d54'
	'github.com/golangci/golangci-lint/v2 v2.12.2'
	'github.com/google/gnostic v0.7.1'
	'github.com/zricethezav/gitleaks/v8 v8.30.1'
  'github.com/securego/gosec/v2 v2.28.0'
  'golang.org/x/vuln v1.7.0'
	'google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.2'
	'google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af'
  'honnef.co/go/tools v0.7.0'
)
for version in "${required_versions[@]}"; do
  rg -Fq "$version" tools/go.mod || {
    echo "missing pinned tool version: $version" >&2
    exit 1
  }
done

rg -Fq 'github.com/go-kratos/kratos/v2 v2.9.2' go.mod || {
  echo "Kratos runtime must be pinned to v2.9.2" >&2
  exit 1
}

for version in 'TRIVY_VERSION:-0.74.0' 'COSIGN_VERSION:-3.1.2'; do
  rg -Fq "$version" scripts/verify-image-supply-chain.sh || {
    echo "missing pinned release tool version: $version" >&2
    exit 1
  }
done

echo "CI policy OK: domestic sources and pinned tools"
