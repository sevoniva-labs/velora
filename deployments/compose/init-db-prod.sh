#!/usr/bin/env bash
set -euo pipefail

: "${POSTGRES_APP_USER:?POSTGRES_APP_USER is required}"
: "${POSTGRES_APP_PASSWORD:?POSTGRES_APP_PASSWORD is required}"
: "${POSTGRES_IDP_USER:?POSTGRES_IDP_USER is required}"
: "${POSTGRES_IDP_PASSWORD:?POSTGRES_IDP_PASSWORD is required}"

if [ "$POSTGRES_APP_USER" = "$POSTGRES_IDP_USER" ]; then
  echo "错误：Velora 与 Casdoor 必须使用不同的 PostgreSQL 账号。" >&2
  exit 1
fi

# 仅在首次初始化空数据目录时执行。使用 psql 的 identifier/literal quoting，
# 允许 Secret 含特殊字符，同时避免把生产凭据写进 SQL 文件。
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=app_user="$POSTGRES_APP_USER" \
  --set=app_password="$POSTGRES_APP_PASSWORD" \
  --set=idp_user="$POSTGRES_IDP_USER" \
  --set=idp_password="$POSTGRES_IDP_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_user', :'app_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'app_user')\gexec

SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'idp_user', :'idp_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'idp_user')\gexec

SELECT format('CREATE DATABASE %I OWNER %I', 'velora', :'app_user')
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'velora')\gexec

SELECT format('CREATE DATABASE %I OWNER %I', 'casdoor', :'idp_user')
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'casdoor')\gexec
SQL

echo "生产数据库初始化完成：velora/casdoor 使用独立应用账号。"
