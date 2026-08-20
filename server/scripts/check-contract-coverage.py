#!/usr/bin/env python3
import pathlib
import re
import sys


ROOT = pathlib.Path(__file__).resolve().parent.parent
METHODS = {"get", "post", "put", "patch", "delete"}


def normalize_path(path: str) -> str:
    return re.sub(r"\{[^}]+\}", "{}", path)


def openapi_operations() -> set[tuple[str, str]]:
    operations: set[tuple[str, str]] = set()
    in_paths = False
    current_path = ""
    path_indent = 0
    for line in (ROOT / "api/gen/openapi/openapi.yaml").read_text(encoding="utf-8").splitlines():
        if line == "paths:":
            in_paths = True
            continue
        if in_paths and line and not line.startswith(" "):
            break
        path_match = re.fullmatch(r"(\s+)(/[^ ]*):", line)
        if path_match:
            path_indent = len(path_match.group(1))
            path = path_match.group(2)
            current_path = normalize_path(path if path.startswith("/api/v1/") else "/api/v1" + path)
            continue
        method_match = re.fullmatch(r"(\s+)([a-z]+):", line)
        if current_path and method_match and len(method_match.group(1)) == path_indent + 4 and method_match.group(2) in METHODS:
            operations.add((method_match.group(2), current_path))
    return operations


def proto_operations() -> set[tuple[str, str]]:
    operations: set[tuple[str, str]] = set()
    # A single RPC can publish multiple bindings (for example an OIDC browser
    # redirect GET alongside the JSON POST callback). Inspect every HTTP verb
    # within the annotation instead of only the first binding.
    pattern = re.compile(r'\b(get|post|put|patch|delete):\s*"([^"]+)"')
    for path in sorted((ROOT / "api/proto/forge").rglob("*.proto")):
        for match in pattern.finditer(path.read_text(encoding="utf-8")):
            operations.add((match.group(1), normalize_path(match.group(2))))
    return operations


def main() -> int:
    openapi = openapi_operations()
    proto = proto_operations()
    missing_proto = sorted(openapi - proto)
    missing_openapi = sorted(proto - openapi)
    if missing_proto or missing_openapi:
        for method, path in missing_proto:
            print(f"missing Proto operation: {method.upper()} {path}", file=sys.stderr)
        for method, path in missing_openapi:
            print(f"missing OpenAPI operation: {method.upper()} {path}", file=sys.stderr)
        return 1
    print(f"contract coverage OK: {len(openapi)} HTTP operations match Proto")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
