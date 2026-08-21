#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

fail() { printf 'AI governance check failed: %s\n' "$1" >&2; exit 1; }

required=(AGENTS.md CLAUDE.md .cursor/rules/forge-banking-scaffold.mdc .agents/skills/forge-banking-scaffold/SKILL.md .agents/skills/forge-banking-scaffold/agents/openai.yaml .claude/skills/forge-banking-scaffold/SKILL.md docs/ai-engineering-governance.md)
for path in "${required[@]}"; do [[ -f "$path" ]] || fail "missing $path"; done

cmp -s .agents/skills/forge-banking-scaffold/SKILL.md .claude/skills/forge-banking-scaffold/SKILL.md || fail 'Claude and Agent Skills copies have drifted'
head -n 1 CLAUDE.md | grep -Fxq '@AGENTS.md' || fail 'CLAUDE.md must import @AGENTS.md first'
grep -Fxq 'alwaysApply: true' .cursor/rules/forge-banking-scaffold.mdc || fail 'Cursor rule must always apply'

for ref in .agents/skills/forge-banking-scaffold/SKILL.md docs/ai-engineering-governance.md docs/banking-production-feasibility-plan.local.md; do
  grep -Fq "$ref" AGENTS.md || fail "AGENTS.md does not load $ref"
done
for ref in AGENTS.md .agents/skills/forge-banking-scaffold/SKILL.md docs/ai-engineering-governance.md; do
  grep -Fq "$ref" .cursor/rules/forge-banking-scaffold.mdc || fail "Cursor rule does not load $ref"
done

if grep -Eq 'Go \+ chi|prefer Transactional Outbox|Transactional Outbox Pattern' AGENTS.md; then fail 'legacy architecture policy remains'; fi

python3 - "$ROOT" <<'PY'
from pathlib import Path
import sys
root = Path(sys.argv[1])
for rel in (".agents/skills/forge-banking-scaffold/SKILL.md", ".claude/skills/forge-banking-scaffold/SKILL.md"):
    text = (root / rel).read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) != 3 or parts[0].strip():
        raise SystemExit(f"AI governance check failed: invalid frontmatter in {rel}")
    keys = {line.split(":", 1)[0].strip() for line in parts[1].splitlines() if line.strip() and not line.lstrip().startswith("#")}
    if keys != {"name", "description"}:
        raise SystemExit(f"AI governance check failed: {rel} frontmatter must contain only name and description")
    if "name: forge-banking-scaffold" not in parts[1]:
        raise SystemExit(f"AI governance check failed: wrong skill name in {rel}")
PY

for phrase in 'Modular monolith' 'Kratos v2' 'Wujie' 'Generic S3 contract' 'Domestic sources and offline mode' 'Target-tested'; do
  grep -Fqi "$phrase" docs/ai-engineering-governance.md || fail "governance policy is missing $phrase"
done

grep -Fq 'GOPROXY ?= https://goproxy.cn' Makefile || fail 'domestic Go proxy default drifted'
grep -Fq 'NPM_REGISTRY ?= https://registry.npmmirror.com' Makefile || fail 'domestic npm registry default drifted'
grep -Fq "GOSUMDB ?= sum.golang.org https://goproxy.cn/sumdb/sum.golang.org" Makefile || fail 'domestic sumdb proxy drifted'
git check-ignore -q docs/banking-production-feasibility-plan.local.md || fail 'local feasibility plan is not ignored'
git check-ignore -q docs/banking-production-goal-prompt.local.md || fail 'local Goal prompt is not ignored'

manifests=(go.mod tools/go.mod)
if [[ -f pnpm-workspace.yaml ]]; then manifests+=(pnpm-workspace.yaml); fi
web_root="$ROOT/web"
if [[ ! -d "$web_root" && -d "$ROOT/../web" ]]; then web_root="$ROOT/../web"; fi
if [[ -d "$web_root" ]]; then
  while IFS= read -r -d '' path; do manifests+=("$path"); done < <(find "$web_root" -name package.json -type f -print0)
fi
forbidden='go-zero|cloudwego/hertz|cloudwego/kitex|apache/dubbo|apache/thrift|seata|qiankun|micro-app|garfish|module-federation|turborepo|(^|/)nx([/@]|$)'
if rg -n -i "$forbidden" "${manifests[@]}"; then fail 'a forbidden direct technology dependency is present'; fi

printf 'AI governance policy is synchronized and enforceable.\n'
