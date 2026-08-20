#!/usr/bin/env bash
# Wave 1 生产 Compose 静态门禁：只解析配置，不启动或修改容器/数据卷。
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE_FILE="deployments/env/prod/docker-compose.yml"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
: >"$tmp_dir/empty.env"

config_json="$tmp_dir/config.json"
env \
  DOCKER_REGISTRY=docker.io \
  VELORA_EXTERNAL_URL=velora.example.com \
  VELORA_DATABASE_URL='postgres://velora_app:dummy@postgres:5432/velora?sslmode=require' \
  CASDOOR_DATA_SOURCE_NAME='user=casdoor_app password=dummy host=postgres port=5432 sslmode=require dbname=casdoor' \
  CASDOOR_CLIENT_ID=velora \
  CASDOOR_CLIENT_SECRET=dummy-client-secret \
  SESSION_SECRET=dummy-session-secret-32-bytes-minimum-000 \
  MAIL_CREDENTIAL_KEY='MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=' \
  REDIS_PASSWORD=dummy-redis-password \
  REDIS_URL='redis://:dummy-redis-password@redis:6379/0' \
  TRUSTED_PROXIES=10.0.0.0/8 \
  POSTGRES_SUPERUSER=postgres_bootstrap \
  POSTGRES_SUPERUSER_PASSWORD=dummy-postgres-password \
  POSTGRES_APP_USER=velora_app \
  POSTGRES_APP_PASSWORD=dummy-app-password \
  POSTGRES_IDP_USER=casdoor_app \
  POSTGRES_IDP_PASSWORD=dummy-idp-password \
  GRAFANA_ADMIN_USER=grafana_admin \
  GRAFANA_ADMIN_PASSWORD=dummy-grafana-password \
  docker compose --env-file "$tmp_dir/empty.env" -f "$COMPOSE_FILE" config --format json >"$config_json"

if ! jq -e '.services.postgres.ports == null and .services.redis.ports == null and .services.casdoor.ports == null and .services.server.ports == null and .services.prometheus.ports == null and .services.grafana.ports == null' "$config_json" >/dev/null; then
  echo "错误：生产 Compose 中非 Web 服务存在 published ports" >&2
  exit 1
fi

if ! jq -e '(.services.web.ports | map(.published | tonumber) | sort) == [80, 443]' "$config_json" >/dev/null; then
  echo "错误：生产 Web 只允许发布 80/443" >&2
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

echo "生产 Compose 静态检查通过：仅 Web 发布 80/443，非 Web 服务无 host port，Casdoor initData=false，无默认凭据。"
