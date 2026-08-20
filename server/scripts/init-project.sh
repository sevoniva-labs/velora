#!/usr/bin/env bash
set -euo pipefail

APP_NAME=${1:?app name required}
MODULE=${2:?go module required}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
OLD_MODULE="github.com/sevoniva-labs/velora/server"

if [[ "$APP_NAME" == *"/"* ]]; then
  echo "APP name must not contain /" >&2
  exit 1
fi

python3 - "$ROOT" "$APP_NAME" "$MODULE" "$OLD_MODULE" <<'PY2'
from pathlib import Path
import sys
root=Path(sys.argv[1]); app=sys.argv[2]; module=sys.argv[3]; old=sys.argv[4]
text_ext={'.go','.mod','.sum','.md','.yaml','.yml','.json','.ts','.tsx','.css','.html','.sh','.service','.tpl'}
for p in root.rglob('*'):
    if not p.is_file() or p.suffix not in text_ext: continue
    if p == root / 'scripts' / 'init-project.sh': continue
    if '.git' in p.parts or 'node_modules' in p.parts: continue
    try: s=p.read_text()
    except UnicodeDecodeError: continue
    s=s.replace(old,module).replace('Velora',app)
    p.write_text(s)
PY2

echo "Initialized project: $APP_NAME ($MODULE)"
echo "Next: cp .env.example .env, fill secrets (including bootstrap + storage credentials), then make compose-up"
