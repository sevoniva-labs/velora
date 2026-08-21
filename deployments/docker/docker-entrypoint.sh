#!/bin/sh
# Velora Web entrypoint：根据 VELORA_TLS_ENABLED 渲染 nginx 配置。
# 放于 /docker-entrypoint.d/，nginx 官方镜像启动时按序执行。
set -e

TEMPLATE_DIR=/etc/nginx/templates
CONF_DIR=/etc/nginx/conf.d

# 仅替换已定义的环境变量（避免破坏 nginx 自身 $host/$uri 等变量）
# 给本地直接运行镜像提供安全的开发默认值；Compose/生产可显式覆盖。
: "${VELORA_API_PROXY_PASS:=http://server:8080}"
: "${VELORA_CASDOOR_PROXY_PASS:=http://casdoor:8000/}"
: "${VELORA_DNS_RESOLVER:=127.0.0.11}"
: "${VELORA_NGINX_RESOLVE_MODE:=dynamic}"
: "${VELORA_FORM_ACTION_SOURCES:=}"

# nerdctl/Lima uses /etc/hosts for service discovery but does not provide
# Docker's 127.0.0.11 resolver. In static mode resolve both service names from
# /etc/hosts at container start; Docker/Compose keeps the dynamic default.
if [ "$VELORA_NGINX_RESOLVE_MODE" = "static" ]; then
  api_host=$(printf '%s' "$VELORA_API_PROXY_PASS" | sed -E 's#^[^:]+://([^/:]+).*#\1#')
  api_ip=$(getent hosts "$api_host" 2>/dev/null | awk 'NR == 1 { print $1 }')
  casdoor_host=$(printf '%s' "$VELORA_CASDOOR_PROXY_PASS" | sed -E 's#^[^:]+://([^/:]+).*#\1#')
  casdoor_ip=$(getent hosts "$casdoor_host" 2>/dev/null | awk 'NR == 1 { print $1 }')
  if [ -z "$api_ip" ] || [ -z "$casdoor_ip" ]; then
    echo "错误：静态服务解析失败（api=$api_host/$api_ip casdoor=$casdoor_host/$casdoor_ip）" >&2
    exit 1
  fi
  VELORA_API_PROXY_PASS=$(printf '%s' "$VELORA_API_PROXY_PASS" | sed "s#//$api_host#//$api_ip#")
  VELORA_CASDOOR_PROXY_PASS=$(printf '%s' "$VELORA_CASDOOR_PROXY_PASS" | sed "s#//$casdoor_host#//$casdoor_ip#")
  # proxy_pass 已是 IP，不会触发 resolver；保留合法值以通过 nginx 配置检查。
  VELORA_DNS_RESOLVER=127.0.0.1
fi

export VELORA_API_PROXY_PASS VELORA_DNS_RESOLVER
export VELORA_CASDOOR_PROXY_PASS VELORA_FORM_ACTION_SOURCES
defined_envs=$(printf '${%s} ' $(env | cut -d= -f1))

render() {
  src="$1"
  dst="$2"
  # 若变量未定义，envsubst 会保留 ${VAR} 字面量；先用 sed 替换为默认值（兼容 POSIX sh 下无 ${VAR:-def} 默认展开）
  sed -E 's/\$\{VELORA_API_PROXY_PASS:-([^}]*)\}/\1/g' "$src" > /tmp/velora_tpl.tmp
  envsubst "$defined_envs" < /tmp/velora_tpl.tmp > "$dst"
  rm -f /tmp/velora_tpl.tmp
}

if [ "${VELORA_TLS_ENABLED:-false}" = "true" ]; then
  echo "==> Velora Web: TLS 模式（80→443 + 443 服务）"
  render "$TEMPLATE_DIR/velora-http-redirect.conf.template" "$CONF_DIR/00-http.conf"
  render "$TEMPLATE_DIR/velora-https.conf.template" "$CONF_DIR/10-https.conf"
  # 证书必须存在，否则 nginx 启动失败并给出明确提示
  if [ ! -f /etc/nginx/certs/velora.crt ] || [ ! -f /etc/nginx/certs/velora.key ]; then
    echo "错误：VELORA_TLS_ENABLED=true 但缺少 /etc/nginx/certs/velora.crt 或 velora.key" >&2
    exit 1
  fi
else
  echo "==> Velora Web: HTTP 模式（80 服务）"
  render "$TEMPLATE_DIR/velora-http.conf.template" "$CONF_DIR/default.conf"
fi
