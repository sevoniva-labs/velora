#!/bin/sh
# Velora Web entrypoint：根据 VELORA_TLS_ENABLED 渲染 nginx 配置。
# 放于 /docker-entrypoint.d/，nginx 官方镜像启动时按序执行。
set -e

TEMPLATE_DIR=/etc/nginx/templates
CONF_DIR=/etc/nginx/conf.d

# 仅替换已定义的环境变量（避免破坏 nginx 自身 $host/$uri 等变量）
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
