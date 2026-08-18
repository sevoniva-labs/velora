#!/usr/bin/env bash
# ============================================================================
# Velora Casdoor 应用初始化脚本（部署期一次性执行）
#
# 在 Casdoor 数据库中创建 Velora 专用的 OIDC 客户端应用（若不存在）：
#   - 启用 grant_types: password + authorization_code
#   - 回调地址: CASDOOR_REDIRECT_URI（默认 http://localhost:8080/api/v1/auth/oidc/callback）
#   - 使用 Casdoor 内置证书（cert-built-in）签发 token
#
# 说明：本脚本是"部署初始化工具"，与 docker-compose 的 init-db.sql 同级；
#       Velora 服务运行时不直连 Casdoor 库（身份数据由 Casdoor 独占管理）。
#
# 用法：
#   ./scripts/init-casdoor.sh                     # 创建并打印 client_id / client_secret
#   CASDOOR_DB_URL=postgres://... ./scripts/init-casdoor.sh
# ============================================================================
set -euo pipefail

# Casdoor 独立 database（与 velora 业务库隔离）
CASDOOR_DB_URL="${CASDOOR_DB_URL:-postgres://postgres:postgres@127.0.0.1:5433/casdoor?sslmode=disable}"
REDIRECT_URI="${CASDOOR_REDIRECT_URI:-http://localhost:8080/api/v1/auth/oidc/callback}"

PSQL="psql"
if ! command -v psql >/dev/null 2>&1 && [ -x /opt/homebrew/bin/psql ]; then
  PSQL="/opt/homebrew/bin/psql"
fi

echo "==> 检查 Casdoor 中是否已存在 velora 应用…"
EXIST=$("$PSQL" "$CASDOOR_DB_URL" -tAc "SELECT count(*) FROM application WHERE name='velora';")
if [ "$EXIST" != "0" ]; then
  echo "==> velora 应用已存在，跳过创建。"
  "$PSQL" "$CASDOOR_DB_URL" -tAc "SELECT client_id || ' ' || client_secret FROM application WHERE name='velora';" \
    | awk '{print "    client_id = " $1 "\n    client_secret = " $2}'
  exit 0
fi

CLIENT_ID=$(openssl rand -hex 10)          # 20 位十六进制（Casdoor 客户端 ID 格式）
CLIENT_SECRET=$(openssl rand -hex 20)      # 40 位十六进制（Casdoor 客户端密钥格式）
NOW=$(date '+%Y-%m-%d %H:%M:%S')
GRANT_TYPES='["password","authorization_code"]'
REDIRECT_JSON=$(printf '"%s"' "$REDIRECT_URI")

echo "==> 创建 velora 应用（owner=admin, org=built-in, cert=cert-built-in）…"
"$PSQL" "$CASDOOR_DB_URL" -v ON_ERROR_STOP=1 <<SQL
INSERT INTO application (
  owner, name, created_time, display_name, logo, homepage_url, description,
  organization, cert, enable_password, enable_signin_session, enable_auto_signin,
  grant_types, client_id, client_secret, redirect_uris, token_format,
  expire_in_hours, refresh_expire_in_hours
) VALUES (
  'admin', 'velora', '$NOW', 'Velora', '', 'http://localhost:5173', 'Velora 企业应用门户',
  'built-in', 'cert-built-in', true, true, false,
  '$GRANT_TYPES', '$CLIENT_ID', '$CLIENT_SECRET', '[$REDIRECT_JSON]', 'JWT',
  168, 168
);
SQL

echo "==> velora 应用创建成功，请将以下凭据填入 .env："
echo "    CASDOOR_CLIENT_ID=$CLIENT_ID"
echo "    CASDOOR_CLIENT_SECRET=$CLIENT_SECRET"
