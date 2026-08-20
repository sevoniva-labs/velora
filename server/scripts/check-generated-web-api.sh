#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

GENERATED=web/packages/api-client/src/generated
if [[ ! -d "$GENERATED" ]]; then
  echo "generated TypeScript API client is missing" >&2
  exit 1
fi

snapshot() {
  find "$GENERATED" -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
}

before=$(snapshot)
corepack pnpm --filter @forge/api-client api:types
after=$(snapshot)
if [[ "$before" != "$after" ]]; then
  echo "generated TypeScript API client is stale; run make web-api-generate and commit the result" >&2
  exit 1
fi

echo "generated TypeScript API client is current"
