#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

export GOPROXY=${GOPROXY:-https://goproxy.cn}
export GOSUMDB="${GOSUMDB:-sum.golang.org https://goproxy.cn/sumdb/sum.golang.org}"
export npm_config_registry=${NPM_REGISTRY:-https://registry.npmmirror.com}
OUT=${EVIDENCE_DIR:-.evidence}
TOOL_RUN=(go run -modfile=tools/go.mod)

case "$GOPROXY" in
  https://goproxy.cn|https://goproxy.io) ;;
  *) echo "GOPROXY must be an approved domestic mirror" >&2; exit 1 ;;
esac
[[ "$npm_config_registry" == "https://registry.npmmirror.com" ]] || {
  echo "NPM_REGISTRY must be https://registry.npmmirror.com" >&2
  exit 1
}

rm -rf "$OUT"
mkdir -p "$OUT"

"${TOOL_RUN[@]}" github.com/anchore/syft/cmd/syft dir:. \
  --exclude './.git/**' --exclude './.evidence/**' \
  -o "cyclonedx-json=$OUT/source.cdx.json"

"${TOOL_RUN[@]}" github.com/zricethezav/gitleaks/v8 git . \
  --config "$ROOT/.gitleaks.toml" \
  --no-banner --redact=100 --report-format json \
  --report-path "$OUT/gitleaks.json"
"${TOOL_RUN[@]}" github.com/zricethezav/gitleaks/v8 dir . \
  --config "$ROOT/.gitleaks.toml" \
  --no-banner --redact=100 --report-format json \
  --report-path "$OUT/gitleaks-worktree.json"

"${TOOL_RUN[@]}" golang.org/x/vuln/cmd/govulncheck -json ./... >"$OUT/govulncheck.stream"
python3 scripts/json-stream-to-array.py "$OUT/govulncheck.stream" "$OUT/govulncheck.json"
rm "$OUT/govulncheck.stream"
corepack pnpm licenses list --prod --recursive --json >"$OUT/web-licenses.json"

{
  printf 'generated_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'commit=%s\n' "$(git rev-parse HEAD)"
  printf 'go=%s\n' "$(go version)"
  printf 'pnpm=%s\n' "$(corepack pnpm --version)"
  printf 'goproxy=%s\n' "$GOPROXY"
  printf 'gosumdb=%s\n' "$GOSUMDB"
  printf 'npm_registry=%s\n' "$npm_config_registry"
  printf 'syft=1.50.0\n'
  printf 'gitleaks=8.30.1\n'
} >"$OUT/provenance.txt"

(
  cd "$OUT"
  shasum -a 256 gitleaks-worktree.json gitleaks.json govulncheck.json provenance.txt source.cdx.json web-licenses.json >SHA256SUMS
)

echo "supply-chain evidence generated in $OUT"
