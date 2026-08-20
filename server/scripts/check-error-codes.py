#!/usr/bin/env python3
"""Fail CI when a literal transport error reason is not registered in codeMap."""
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
codes = (root / "internal/platform/httpx/codes.go").read_text(encoding="utf-8")
registered = set(re.findall(r'"([A-Z][A-Z0-9_]+)"\s*:\s*"\d{6}"', codes))
used = set()
for path in (root / "internal").rglob("*.go"):
    text = path.read_text(encoding="utf-8")
    used.update(re.findall(r'httpx\.Error\([^\n]*?"([A-Z][A-Z0-9_]+)"', text))
    used.update(re.findall(r'(?:kerrors|kratoserrors)\.(?:BadRequest|Unauthorized|Forbidden|NotFound|Conflict|InternalServer|ServiceUnavailable)\(\s*"([A-Z][A-Z0-9_]+)"', text))
    used.update(re.findall(r'(?:kerrors|kratoserrors)\.New\([^,]+,\s*"([A-Z][A-Z0-9_]+)"', text))
missing = sorted(used - registered)
if missing:
    print("unregistered error symbols:", ", ".join(missing), file=sys.stderr)
    raise SystemExit(1)
print(f"error-code contract OK: {len(used)} literal symbols are registered")
