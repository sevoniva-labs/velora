#!/usr/bin/env python3
"""Verify generated OpenAPI authentication and CSRF declarations."""
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[1]
SPEC = ROOT / "api/gen/openapi/openapi.yaml"
METHODS = {"get", "post", "put", "patch", "delete"}
PUBLIC = {
    ("get", "/api/v1/system/health"),
    ("get", "/api/v1/system/ready"),
    ("post", "/api/v1/auth/login"),
    ("get", "/api/v1/auth/federated/oidc/{provider}/begin"),
    ("post", "/api/v1/auth/federated/oidc/{provider}/callback"),
    ("get", "/api/v1/auth/federated/oidc/{provider}/callback"),
    ("post", "/api/v1/auth/federated/ldap/{provider}"),
}
CSRF_REQUIRED = {
    ("post", "/api/v1/auth/logout"),
    ("patch", "/api/v1/auth/password"),
    ("post", "/api/v1/auth/step-up"),
    ("post", "/api/v1/mfa/totp/enrollment"),
    ("post", "/api/v1/mfa/totp/enrollment/confirmation"),
    ("post", "/api/v1/mfa/totp/disable"),
    ("post", "/api/v1/api-tokens"),
    ("delete", "/api/v1/api-tokens/{token_id}"),
    ("post", "/api/v1/admin/users"),
    ("post", "/api/v1/admin/departments"),
    ("patch", "/api/v1/admin/departments/{department_id}"),
    ("post", "/api/v1/admin/positions"),
    ("patch", "/api/v1/admin/positions/{position_id}"),
    ("post", "/api/v1/admin/user-groups"),
    ("patch", "/api/v1/admin/user-groups/{group_id}"),
    ("put", "/api/v1/admin/user-groups/{group_id}/members"),
    ("put", "/api/v1/admin/user-groups/{group_id}/roles"),
    ("put", "/api/v1/admin/users/{user_id}/assignments"),
    ("patch", "/api/v1/admin/organization"),
    ("put", "/api/v1/admin/security-config"),
    ("put", "/api/v1/admin/roles/{role_key}/permissions"),
    ("put", "/api/v1/admin/roles/{role_key}/data-scope"),
    ("patch", "/api/v1/admin/users/{user_id}/roles"),
    ("patch", "/api/v1/admin/users/{user_id}/status"),
    ("post", "/api/v1/admin/users/{user_id}/unlock"),
    ("post", "/api/v1/admin/users/{user_id}/reset-password"),
    ("delete", "/api/v1/admin/sessions/{session_id}"),
    ("get", "/api/v1/admin/audit-logs/export"),
    ("post", "/api/v1/admin/temporary-role-grants"),
    ("post", "/api/v1/admin/temporary-role-grants/{grant_id}:revoke"),
    ("post", "/api/v1/admin/identity-mappings"),
    ("post", "/api/v1/admin/identity-mappings/{link_id}:unlink"),
    ("post", "/api/v1/admin/access-reviews"),
    ("post", "/api/v1/admin/access-reviews/{review_id}/items/{item_id}/decisions"),
    ("patch", "/api/v1/admin/menus/{menu_key}"),
    ("put", "/api/v1/admin/data-policies/{field_key}"),
    ("post", "/api/v1/admin/data-exports/authorize"),
    ("post", "/api/v1/admin/data-retention/evidence"),
    ("post", "/api/v1/admin/config-changes"),
    ("post", "/api/v1/admin/config-changes/{change_id}/approve"),
    ("post", "/api/v1/admin/config-changes/{change_id}/publish"),
    ("post", "/api/v1/admin/config-changes/{change_id}/rollback-request"),
    ("post", "/api/v1/admin/config-changes/{change_id}/rollback"),
    ("post", "/api/v1/admin/portal/applications"),
    ("patch", "/api/v1/admin/portal/applications/{application_id}"),
    ("delete", "/api/v1/admin/portal/applications/{application_id}"),
    ("post", "/api/v1/admin/portal/categories"),
    ("patch", "/api/v1/admin/portal/categories/{category_id}"),
    ("delete", "/api/v1/admin/portal/categories/{category_id}"),
    ("post", "/api/v1/admin/portal/tags"),
    ("patch", "/api/v1/admin/portal/tags/{tag_id}"),
    ("delete", "/api/v1/admin/portal/tags/{tag_id}"),
    ("put", "/api/v1/admin/portal/applications/{application_id}/policies"),
    ("post", "/api/v1/portal/applications/{application_id}/launch"),
    ("post", "/api/v1/portal/favorites"),
    ("delete", "/api/v1/portal/favorites/{application_id}"),
    ("post", "/api/v1/approvals"),
    ("post", "/api/v1/approvals/{approval_id}/decisions"),
    ("post", "/api/v1/approvals/{approval_id}/transfer"),
    ("post", "/api/v1/approvals/{approval_id}/withdraw"),
}
INTERACTIVE_ONLY = {
    ("post", "/api/v1/auth/logout"),
    ("patch", "/api/v1/auth/password"),
    ("get", "/api/v1/api-tokens"),
    ("post", "/api/v1/api-tokens"),
    ("delete", "/api/v1/api-tokens/{token_id}"),
}


def normalize_path(path: str) -> str:
    return re.sub(r"\{[^}]+\}", "{}", path)


def normalize_policy(policy: set[tuple[str, str]]) -> set[tuple[str, str]]:
    return {(method, normalize_path(path)) for method, path in policy}


def operations(lines: list[str]) -> dict[tuple[str, str], str]:
    result: dict[tuple[str, str], str] = {}
    current_path = ""
    current_key: tuple[str, str] | None = None
    block: list[str] = []
    for line in lines:
        path_match = re.fullmatch(r"    (/.*):", line)
        if path_match:
            if current_key:
                result[current_key] = "\n".join(block)
            current_path = normalize_path(path_match.group(1))
            current_key = None
            block = []
            continue
        method_match = re.fullmatch(r"        ([a-z]+):", line)
        if current_path and method_match and method_match.group(1) in METHODS:
            if current_key:
                result[current_key] = "\n".join(block)
            current_key = (method_match.group(1), current_path)
            block = []
            continue
        if current_key:
            block.append(line)
    if current_key:
        result[current_key] = "\n".join(block)
    return result


def main() -> int:
    text = SPEC.read_text(encoding="utf-8")
    for scheme in ("SessionCookie:", "BearerAuth:", "CsrfToken:"):
        if scheme not in text:
            print(f"missing OpenAPI security scheme: {scheme[:-1]}", file=sys.stderr)
            return 1

    parsed = operations(text.splitlines())
    public = normalize_policy(PUBLIC)
    csrf_required = normalize_policy(CSRF_REQUIRED)
    interactive_only = normalize_policy(INTERACTIVE_ONLY)
    failures: list[str] = []
    for key, block in sorted(parsed.items()):
        has_security = "            security:" in block
        has_csrf = "CsrfToken: []" in block
        has_bearer = "BearerAuth: []" in block
        if key in public and has_security:
            failures.append(f"public operation unexpectedly secured: {key[0].upper()} {key[1]}")
        if key not in public and not has_security:
            failures.append(f"protected operation missing security: {key[0].upper()} {key[1]}")
        if (key in csrf_required) != has_csrf:
            failures.append(f"CSRF policy mismatch: {key[0].upper()} {key[1]}")
        if key in interactive_only and has_bearer:
            failures.append(f"interactive-only operation permits Bearer: {key[0].upper()} {key[1]}")
    missing = (public | csrf_required | interactive_only) - parsed.keys()
    for method, path in sorted(missing):
        failures.append(f"security policy references missing operation: {method.upper()} {path}")
    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1
    print(f"OpenAPI security contract OK: {len(parsed)} operations, {len(csrf_required)} CSRF-protected")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
