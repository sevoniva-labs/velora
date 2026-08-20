#!/usr/bin/env bash
# 本地 Casdoor OIDC 授权码 + PKCE 验收。
#
# 该脚本只使用环境变量中的测试凭据，不创建/修改 Casdoor 应用和用户。
# 运行前必须先完成 Velora external identity 的审批绑定；脚本不会绕过审批。
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

VELORA_BASE_URL="${VELORA_BASE_URL:-http://localhost:8080}"
CASDOOR_BASE_URL="${CASDOOR_BASE_URL:-http://localhost:8443}"
VELORA_OIDC_CLIENT_ID="${VELORA_OIDC_CLIENT_ID:-}"
VELORA_OIDC_CLIENT_SECRET="${VELORA_OIDC_CLIENT_SECRET:-}"
CASDOOR_TEST_USERNAME="${CASDOOR_TEST_USERNAME:-}"
CASDOOR_TEST_PASSWORD="${CASDOOR_TEST_PASSWORD:-}"
CASDOOR_APPLICATION="${CASDOOR_APPLICATION:-velora}"
CASDOOR_ORGANIZATION="${CASDOOR_ORGANIZATION:-built-in}"
CASDOOR_REDIRECT_URI="${CASDOOR_REDIRECT_URI:-http://localhost:5173/auth/callback}"
EVIDENCE_DIR="${VELORA_ACCEPTANCE_EVIDENCE_DIR:-./artifacts/acceptance}"

for required in VELORA_OIDC_CLIENT_ID VELORA_OIDC_CLIENT_SECRET CASDOOR_TEST_USERNAME CASDOOR_TEST_PASSWORD; do
  if [[ -z "${!required}" ]]; then
    echo "错误：${required} 必须通过环境变量提供，脚本不接受硬编码凭据" >&2
    exit 2
  fi
done
if ! command -v curl >/dev/null 2>&1 || ! command -v python3 >/dev/null 2>&1; then
  echo "错误：需要 curl 与 python3" >&2
  exit 2
fi

mkdir -p "$EVIDENCE_DIR"
tmp_dir="$(mktemp -d -t velora-casdoor-oidc.XXXXXX)"
trap 'rm -rf "$tmp_dir"' EXIT
cookie_jar="$tmp_dir/cookies.txt"
begin_json="$tmp_dir/begin.json"
casdoor_json="$tmp_dir/casdoor.json"
callback_json="$tmp_dir/callback.json"
report="$EVIDENCE_DIR/casdoor-oidc-$(date -u +%Y%m%dT%H%M%SZ).json"

status="failed"
failure=""
cleanup_report() {
  python3 - "$report" "$status" "$failure" <<'PY'
import json, sys
path, status, failure = sys.argv[1:]
with open(path, "w", encoding="utf-8") as stream:
    json.dump({"schema": "velora.acceptance.casdoor-oidc.v1", "status": status, "failure": failure or None}, stream, ensure_ascii=False, indent=2)
    stream.write("\n")
PY
}
fail() {
  failure="$1"
  cleanup_report
  echo "Casdoor OIDC smoke failed: $failure" >&2
  exit 1
}

begin_status="$(curl --silent --show-error --fail-with-body -c "$cookie_jar" -o "$begin_json" -w '%{http_code}' \
  "$VELORA_BASE_URL/api/v1/auth/federated/oidc/casdoor/begin?organization=default")" || fail "Velora begin 请求失败"
[[ "$begin_status" == 2* ]] || fail "Velora begin HTTP $begin_status"

redirect_url="$(python3 - "$begin_json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
payload = data.get("data", data)
url = str(payload.get("redirect_url", payload.get("redirectUrl", "")))
if not url:
    raise SystemExit("missing redirectUrl")
print(url)
PY
)" || fail "begin 响应缺少 redirectUrl"

oidc_values="$(python3 - "$redirect_url" <<'PY'
from urllib.parse import parse_qs, urlparse
import sys
query = parse_qs(urlparse(sys.argv[1]).query)
aliases = (("clientId", "client_id"), ("redirectUri", "redirect_uri"), ("state",), ("nonce",), ("code_challenge",), ("code_challenge_method",))
for names in aliases:
    values = next((query.get(name, []) for name in names if query.get(name)), [])
    if not values:
        raise SystemExit(f"missing {key}")
    print(values[0], end="\t")
PY
)" || fail "OIDC 授权地址缺少必要参数"
IFS=$'\t' read -r client_id redirect_uri state nonce code_challenge code_challenge_method _ <<< "$oidc_values"
[[ "$client_id" == "$VELORA_OIDC_CLIENT_ID" ]] || fail "授权地址 client_id 与配置不一致"

login_body="$(python3 - "$CASDOOR_ORGANIZATION" "$CASDOOR_TEST_USERNAME" "$CASDOOR_TEST_PASSWORD" "$CASDOOR_APPLICATION" <<'PY'
import json, sys
print(json.dumps({"type": "code", "organization": sys.argv[1], "username": sys.argv[2], "password": sys.argv[3], "application": sys.argv[4], "signinMethod": "Password"}))
PY
)"
login_url="$(python3 - "$CASDOOR_BASE_URL/api/login" "$client_id" "$redirect_uri" "$state" "$nonce" "$code_challenge" "$code_challenge_method" <<'PY'
from urllib.parse import urlencode
import sys
base, client_id, redirect_uri, state, nonce, challenge, method = sys.argv[1:]
query = {"clientId": client_id, "responseType": "code", "redirectUri": redirect_uri, "scope": "openid profile email", "state": state, "nonce": nonce, "code_challenge_method": method, "code_challenge": challenge, "type": "code"}
print(base + "?" + urlencode(query))
PY
)"
casdoor_status="$(curl --silent --show-error --fail-with-body -o "$casdoor_json" -w '%{http_code}' -X POST \
  "$login_url" \
  -H 'Content-Type: application/json' --data-binary "$login_body")" || fail "Casdoor 授权请求失败"
[[ "$casdoor_status" == 2* ]] || fail "Casdoor 授权 HTTP $casdoor_status"
code="$(python3 - "$casdoor_json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
code = str(data.get("code", data.get("data", "")))
if not code:
    raise SystemExit("missing authorization code")
print(code)
PY
)" || fail "Casdoor 未返回授权码"

callback_body="$(python3 - "$code" "$state" <<'PY'
import json, sys
print(json.dumps({"code": sys.argv[1], "state": sys.argv[2]}))
PY
)"
callback_status="$(curl --silent --show-error --fail-with-body -b "$cookie_jar" -c "$cookie_jar" -o "$callback_json" -w '%{http_code}' \
  -X POST -H 'Content-Type: application/json' --data-binary "$callback_body" \
  "$VELORA_BASE_URL/api/v1/auth/federated/oidc/casdoor/callback")" || fail "Velora callback 请求失败"
[[ "$callback_status" == 2* ]] || fail "Velora callback HTTP $callback_status（检查 Casdoor token endpoint 与 external identity 审批绑定）"

me_status="$(curl --silent --show-error -b "$cookie_jar" -o /dev/null -w '%{http_code}' "$VELORA_BASE_URL/api/v1/me")"
[[ "$me_status" == 2* ]] || fail "OIDC 登录后 /me 未认证（HTTP $me_status）"

replay_status="$(curl --silent --show-error -b "$cookie_jar" -o /dev/null -w '%{http_code}' \
  -X POST -H 'Content-Type: application/json' --data-binary "$callback_body" \
  "$VELORA_BASE_URL/api/v1/auth/federated/oidc/casdoor/callback")"
[[ "$replay_status" != 2* ]] || fail "同一 state/code 被重复接受，未满足一次性消费"

csrf_token="$(awk '$6 == "velora_csrf" {print $7}' "$cookie_jar" | tail -1)"
logout_status="$(curl --silent --show-error -b "$cookie_jar" -c "$cookie_jar" -o /dev/null -w '%{http_code}' \
  -X POST -H 'Content-Type: application/json' -H "X-CSRF-Token: $csrf_token" --data '{}' \
  "$VELORA_BASE_URL/api/v1/auth/logout")"
[[ "$logout_status" == 2* ]] || fail "退出登录 HTTP $logout_status"
after_logout_status="$(curl --silent --show-error -b "$cookie_jar" -o /dev/null -w '%{http_code}' "$VELORA_BASE_URL/api/v1/me")"
[[ "$after_logout_status" != 2* ]] || fail "退出登录后会话仍有效"

status="passed"
cleanup_report
python3 - "$report" "$begin_status" "$casdoor_status" "$callback_status" "$me_status" "$replay_status" "$logout_status" "$after_logout_status" <<'PY'
import json, sys
path, *values = sys.argv[1:]
with open(path, encoding="utf-8") as stream:
    report = json.load(stream)
report.update({"checks": {"begin_http": values[0], "casdoor_authorize_http": values[1], "callback_http": values[2], "me_http": values[3], "replay_http": values[4], "logout_http": values[5], "after_logout_me_http": values[6]}})
with open(path, "w", encoding="utf-8") as stream:
    json.dump(report, stream, ensure_ascii=False, indent=2)
    stream.write("\n")
PY
echo "Casdoor OIDC smoke passed; evidence=$report"
