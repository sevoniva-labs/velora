#!/usr/bin/env bash
# Wave 1 生产 Compose 静态门禁：只解析配置，不启动或修改容器/数据卷。
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE_FILE="deployments/env/prod/docker-compose.yml"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
: >"$tmp_dir/empty.env"
printf 'dummy\n' >"$tmp_dir/redis-ca.pem"
printf 'dummy\n' >"$tmp_dir/redis-cert.pem"
printf 'dummy\n' >"$tmp_dir/redis-key.pem"
printf 'dummy-crypto-key-material-32-bytes\n' >"$tmp_dir/crypto.key"
printf 'dummy-bootstrap-password\n' >"$tmp_dir/bootstrap.password"
printf 'dummy-client-secret\n' >"$tmp_dir/oidc-client.secret"
printf 'dummy-redis-password\n' >"$tmp_dir/redis.password"
printf 'postgres://velora_app:dummy@postgres:5432/velora?sslmode=verify-full\n' >"$tmp_dir/database.dsn"
printf 'dummy-storage-access\n' >"$tmp_dir/storage.access"
printf 'dummy-storage-secret\n' >"$tmp_dir/storage.secret"

config_json="$tmp_dir/config.json"
env \
  DOCKER_REGISTRY=docker.io \
  POSTGRES_IMAGE=harbor.internal.example/approved/postgres@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  REDIS_IMAGE=harbor.internal.example/approved/redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
  CASDOOR_IMAGE=harbor.internal.example/approved/casdoor@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc \
  PROMETHEUS_IMAGE=harbor.internal.example/approved/prometheus@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd \
  GRAFANA_IMAGE=harbor.internal.example/approved/grafana@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee \
  VELORA_EXTERNAL_URL=velora.example.com \
  VELORA_DATABASE_DSN_FILE="$tmp_dir/database.dsn" \
  VELORA_STORAGE_PROVIDER=s3-compatible \
  VELORA_STORAGE_ENDPOINT=https://objects.example.internal \
  VELORA_STORAGE_REGION=cn-north-1 \
  VELORA_STORAGE_BUCKET=velora-prod \
  VELORA_STORAGE_PREFIX=velora-prod \
  VELORA_STORAGE_PATH_STYLE=true \
  VELORA_STORAGE_ACCESS_KEY_FILE="$tmp_dir/storage.access" \
  VELORA_STORAGE_SECRET_KEY_FILE="$tmp_dir/storage.secret" \
  VELORA_STORAGE_SSE_MODE=kms \
  VELORA_STORAGE_SSE_KMS_KEY_ID=dummy-kms-key \
  VELORA_CRYPTO_PROVIDER=standard \
  VELORA_CRYPTO_ADAPTER=hsm \
  VELORA_CRYPTO_KEY_VERSION=v1 \
  VELORA_CRYPTO_KEY_FILE="$tmp_dir/crypto.key" \
  VELORA_AUTH_MODE=oidc \
  VELORA_OIDC_ISSUER=https://casdoor.example.com \
  VELORA_OIDC_CLIENT_ID=velora \
  VELORA_OIDC_REDIRECT_URL=https://velora.example.com/auth/callback \
  VELORA_OIDC_POST_LOGOUT_REDIRECT_URL=https://velora.example.com/login \
  VELORA_SESSION_TTL=1h \
  VELORA_CASDOOR_ACCOUNT_URL=https://casdoor.example.com/account \
  VELORA_OIDC_PROVIDER_ENABLED=false \
  VELORA_BOOTSTRAP_ADMIN=break-glass \
  VELORA_BOOTSTRAP_PASSWORD_FILE="$tmp_dir/bootstrap.password" \
  VELORA_ALLOWED_ORIGINS=https://velora.example.com \
  CASDOOR_DATA_SOURCE_NAME='user=casdoor_app password=dummy host=postgres port=5432 sslmode=require dbname=casdoor' \
  CASDOOR_CLIENT_ID=velora \
  CASDOOR_CLIENT_SECRET=dummy-client-secret \
  SESSION_SECRET=dummy-session-secret-32-bytes-minimum-000 \
  MAIL_CREDENTIAL_KEY='MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=' \
  REDIS_PASSWORD_FILE="$tmp_dir/redis.password" \
  REDIS_TLS_CA_FILE="$tmp_dir/redis-ca.pem" \
  REDIS_TLS_CERT_FILE="$tmp_dir/redis-cert.pem" \
  REDIS_TLS_KEY_FILE="$tmp_dir/redis-key.pem" \
  CASDOOR_OIDC_CLIENT_SECRET_FILE="$tmp_dir/oidc-client.secret" \
  TRUSTED_PROXIES=10.0.0.0/8 \
  POSTGRES_SUPERUSER=postgres_bootstrap \
  POSTGRES_SUPERUSER_PASSWORD=dummy-postgres-password \
  POSTGRES_APP_USER=velora_app \
  POSTGRES_APP_PASSWORD=dummy-app-password \
  POSTGRES_IDP_USER=casdoor_app \
  POSTGRES_IDP_PASSWORD=dummy-idp-password \
  GRAFANA_ADMIN_USER=grafana_admin \
  GRAFANA_ADMIN_PASSWORD=dummy-grafana-password \
  docker compose --env-file "$tmp_dir/empty.env" --profile monitoring -f "$COMPOSE_FILE" config --format json >"$config_json"

if ! jq -e '.services.postgres.ports == null and .services.redis.ports == null and .services.casdoor.ports == null and .services.server.ports == null and .services.prometheus.ports == null and .services.grafana.ports == null' "$config_json" >/dev/null; then
  echo "错误：生产 Compose 中非 Web 服务存在 published ports" >&2
  exit 1
fi

if ! jq -e '[.services.postgres.image, .services.redis.image, .services.casdoor.image, .services.prometheus.image, .services.grafana.image] | length == 5 and all(.[]; type == "string" and test("@sha256:[0-9a-f]{64}$"))' "$config_json" >/dev/null; then
  echo "错误：生产基础设施镜像必须全部固定为内部 digest" >&2
  exit 1
fi

if ! jq -e '.services.server.build.context | endswith("/velora")' "$config_json" >/dev/null || ! jq -e '.services.web.build.context | endswith("/velora")' "$config_json" >/dev/null; then
  echo "错误：生产 server/web 构建上下文必须指向仓库根目录" >&2
  exit 1
fi

if ! jq -e '(.services.web.ports | map(.published | tonumber) | sort) == [80, 443]' "$config_json" >/dev/null; then
  echo "错误：生产 Web 只允许发布 80/443" >&2
  exit 1
fi

if ! jq -e '.services.web.environment.VELORA_TLS_ENABLED == "true" and (.services.web.volumes | any(.type == "bind" and .target == "/etc/nginx/certs" and .read_only == true))' "$config_json" >/dev/null; then
  echo "错误：生产 Web 必须启用 TLS 并以只读方式挂载证书目录" >&2
  exit 1
fi

if jq -e '.. | strings | select(test("postgres/postgres|admin/admin|admin/123|change-me|docker-compose-dev-secret"))' "$config_json" >/dev/null; then
  echo "错误：生产 Compose 中发现默认凭据或占位 Secret" >&2
  exit 1
fi

if ! jq -e '.services.casdoor.environment.initData == "false"' "$config_json" >/dev/null; then
  echo "错误：生产 Casdoor 必须禁用 initData" >&2
  exit 1
fi

if ! jq -e '.services.server.environment.VELORA_OIDC_PROVIDER_ENABLED == "false"' "$config_json" >/dev/null; then
  echo "错误：生产 server 必须显式禁用 Velora 自建 OIDC Provider" >&2
  exit 1
fi

if ! jq -e '.services.server.environment.VELORA_CRYPTO_ADAPTER | length > 0' "$config_json" >/dev/null; then
  echo "错误：生产 server 必须显式配置 crypto adapter（software 仅适用于非生产基线）" >&2
  exit 1
fi

if ! jq -e '.services.server.environment.VELORA_STORAGE_SSE_MODE == "kms" and (.services.server.environment.VELORA_STORAGE_SSE_KMS_KEY_ID | length > 0)' "$config_json" >/dev/null; then
  echo "错误：生产对象存储必须启用 KMS SSE 并配置 key id" >&2
  exit 1
fi

if ! jq -e '.services.server.environment.VELORA_DATABASE_DSN == null and .services.server.environment.VELORA_DATABASE_DSN_FILE == "/run/secrets/velora_database_dsn" and .services.server.environment.VELORA_STORAGE_ACCESS_KEY == null and .services.server.environment.VELORA_STORAGE_ACCESS_KEY_FILE == "/run/secrets/storage_access_key" and .services.server.environment.VELORA_STORAGE_SECRET_KEY == null and .services.server.environment.VELORA_STORAGE_SECRET_KEY_FILE == "/run/secrets/storage_secret_key"' "$config_json" >/dev/null; then
  echo "错误：生产 server 的数据库 DSN 与对象存储凭据必须只读 Secret 文件注入" >&2
  exit 1
fi
if ! jq -e '.services.redis.environment.REDIS_PASSWORD == null and (.services.redis.secrets | any((.source // .) == "redis_password")) and (.services.server.secrets | any((.source // .) == "redis_password"))' "$config_json" >/dev/null; then
  echo "错误：生产 Redis 密码不得通过环境变量注入，且必须挂载 Secret 文件" >&2
  exit 1
fi

if jq -e '.. | strings | select(test("/api/v1/auth/federated/oidc/.*/callback"))' "$config_json" >/dev/null; then
  echo "错误：生产 Compose 仍使用后端 OIDC callback，必须使用 SPA /auth/callback" >&2
  exit 1
fi

for nginx_template in deployments/docker/velora-http.conf.template deployments/docker/velora-https.conf.template; do
  if rg -Fq 'return 200 "ok\\n"' "$nginx_template" || ! rg -Fq '/api/v1/system/ready' "$nginx_template"; then
    echo "错误：$nginx_template 的 /healthz 必须代理到 server readiness，不得固定返回 200" >&2
    exit 1
  fi
  for header in X-Velora-Authenticated X-Velora-Application-ID X-Velora-User-ID X-Velora-Login-Name X-Velora-Organization-ID; do
    rg -Fq "proxy_set_header $header \"\";" "$nginx_template" || {
      echo "错误：$nginx_template 未清理入站 $header" >&2
      exit 1
    }
  done
done
if ! rg -Fq 'Strict-Transport-Security' deployments/docker/velora-https.conf.template; then
  echo "错误：生产 HTTPS 模板必须启用 HSTS" >&2
  exit 1
fi

echo "生产 Compose 静态检查通过：仅 Web 发布 80/443，非 Web 服务无 host port，Casdoor initData=false，无默认凭据。"
