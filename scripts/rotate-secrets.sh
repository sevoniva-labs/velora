#!/usr/bin/env bash
# ============================================================================
# Velora 密钥轮换脚本（Phase A5）
#
# 支持：
#   ./scripts/rotate-secrets.sh new-session-secret    # 轮换 SESSION_SECRET
#   ./scripts/rotate-secrets.sh new-mail-key          # 轮换 MAIL_CREDENTIAL_KEY
#   ./scripts/rotate-secrets.sh --generate            # 生成全部新密钥并打印（不写入）
#
# 影响：
#   - SESSION_SECRET 轮换：所有已登录会话立即失效（无状态 HMAC 会话），
#     需在低峰窗口执行并通知用户重新登录。
#   - MAIL_CREDENTIAL_KEY 轮换：已加密存储的邮箱凭证将无法解密，
#     用户需重新绑定邮箱账号（解除绑定 → 重新绑定）。
#     ——因此邮件密钥轮换必须评估业务影响，建议只在凭证泄露或定期合规轮换时执行。
# ============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE=".env"

usage() {
  echo "用法："
  echo "  $0 new-session-secret <新密钥(≥32字节)>   轮换 SESSION_SECRET"
  echo "  $0 new-mail-key <新密钥(base64 32字节)>     轮换 MAIL_CREDENTIAL_KEY"
  echo "  $0 --generate                               生成两组新密钥（仅打印，不写入）"
  echo "  $0 --validate                               校验 .env 密钥格式"
}

update_env() {
  local key="$1" value="$2"
  if ! grep -q "^${key}=" "$ENV_FILE"; then
    echo "${key}=${value}" >> "$ENV_FILE"
    echo "==> 已追加 ${key} 到 $ENV_FILE"
    return
  fi
  sed -i.bak "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
  rm -f "$ENV_FILE.bak"
  echo "==> 已更新 ${key}"
}

case "${1:-}" in
  --generate)
    echo "新 SESSION_SECRET（hex 32 字节）："
    openssl rand -hex 32
    echo "新 MAIL_CREDENTIAL_KEY（base64 32 字节）："
    openssl rand -base64 32
    ;;
  new-session-secret)
    [ $# -ge 2 ] || { usage; exit 1; }
    SECRET="$2"
    [ ${#SECRET} -ge 32 ] || { echo "错误：SESSION_SECRET 至少 32 字节" >&2; exit 1; }
    echo "警告：轮换 SESSION_SECRET 将使所有已登录用户立即失效（需重新登录）。确认继续？[yes/no]"
    read -r CONFIRM
    [ "$CONFIRM" = "yes" ] || { echo "已取消"; exit 0; }
    update_env "SESSION_SECRET" "$SECRET"
    echo "==> 请重启 server 生效：docker compose up -d server（生产：make docker-up-prod）"
    ;;
  new-mail-key)
    [ $# -ge 2 ] || { usage; exit 1; }
    KEY="$2"
    # 校验 base64 解码后为 32 字节
    DECODED_LEN=$(echo "$KEY" | base64 -d 2>/dev/null | wc -c | tr -d ' ')
    [ "$DECODED_LEN" = "32" ] || { echo "错误：MAIL_CREDENTIAL_KEY 必须为 base64 编码的 32 字节密钥（openssl rand -base64 32）" >&2; exit 1; }
    echo "警告：轮换 MAIL_CREDENTIAL_KEY 后，已绑定的邮箱账号凭证将无法解密，用户需重新绑定。确认继续？[yes/no]"
    read -r CONFIRM
    [ "$CONFIRM" = "yes" ] || { echo "已取消"; exit 0; }
    update_env "MAIL_CREDENTIAL_KEY" "$KEY"
    echo "==> 请重启 server 生效；并通知用户重新绑定邮箱账号"
    ;;
  --validate)
    echo "==> 校验 SESSION_SECRET…"
    SESSION_SECRET=$(grep -E '^SESSION_SECRET=' "$ENV_FILE" | cut -d= -f2-)
    [ -n "$SESSION_SECRET" ] && [ ${#SESSION_SECRET} -ge 32 ] \
      && echo "  SESSION_SECRET: OK (${#SESSION_SECRET} 字节)" \
      || { echo "  SESSION_SECRET: 缺失或不足 32 字节" >&2; }
    echo "==> 校验 MAIL_CREDENTIAL_KEY（存在即可，格式由 server 启动校验）…"
    grep -qE '^MAIL_CREDENTIAL_KEY=.+' "$ENV_FILE" \
      && echo "  MAIL_CREDENTIAL_KEY: 已配置" \
      || echo "  MAIL_CREDENTIAL_KEY: 未配置（开发环境允许，生产启动会强制校验）"
    ;;
  *)
    usage
    exit 1
    ;;
esac
