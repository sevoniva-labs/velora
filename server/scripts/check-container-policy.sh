#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

required=(
  'FROM ${NODE_IMAGE}'
  'FROM ${GO_IMAGE}'
  'FROM ${RUNTIME_IMAGE}'
  'COPY package.json pnpm-workspace.yaml pnpm-lock.yaml .npmrc'
  'pnpm install --frozen-lockfile'
  'COPY go.mod go.sum'
  'go mod download && go mod verify'
)
for text in "${required[@]}"; do
  rg -Fq "$text" deploy/docker/Dockerfile || {
    echo "Dockerfile policy missing: $text" >&2
    exit 1
  }
done

for forbidden in 'apt-get' 'docker.io' 'ghcr.io' ':latest'; do
  if rg -Fq "$forbidden" deploy/docker/Dockerfile deploy/compose; then
    echo "container configuration contains forbidden floating or online dependency: $forbidden" >&2
    exit 1
  fi
done

if rg -n '(^|[[:space:]])npm[[:space:]]+(install|ci)([[:space:]]|$)' deploy/docker/Dockerfile; then
  echo "Dockerfile must use frozen pnpm installation, not npm install/ci" >&2
  exit 1
fi

if rg -n '^\s*image:\s+[^$]' deploy/compose --glob '*.{yaml,yml}'; then
  echo "Compose images must be supplied as internal digest variables" >&2
  exit 1
fi

while IFS= read -r image_line; do
  [[ "$image_line" == *':?'* ]] || {
    echo "Compose image variables must fail when unset: $image_line" >&2
    exit 1
  }
done < <(rg '^\s*image:' deploy/compose --glob '*.{yaml,yml}')

if rg -n '^FROM[[:space:]]+[^$]' deploy/docker/Dockerfile; then
  echo "every base image must be supplied as an internal digest build argument" >&2
  exit 1
fi

echo "container policy OK: locked dependencies and digest-only internal bases"
