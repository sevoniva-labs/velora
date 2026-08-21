# Casdoor-backed Velora 登录表单

本地 Compose 默认启用 `VELORA_CASDOOR_PASSWORD_LOGIN_ENABLED=true`：用户只看到 Velora 登录页，账号和密码通过 Velora 后端发送到 Casdoor 的应用登录接口校验；Casdoor 返回一次性 authorization code，Velora 再用现有 OIDC client secret 换取并校验 ID Token，最后按已批准的 `external_identities` 映射建立 Velora session。

## 配置

```dotenv
VELORA_AUTH_MODE=oidc
VELORA_OIDC_ISSUER=http://localhost:8443
VELORA_OIDC_INTERNAL_URL=http://casdoor:8000
VELORA_OIDC_CLIENT_ID=<Casdoor application client id>
VELORA_OIDC_CLIENT_SECRET=<secret or *_FILE>
VELORA_OIDC_REDIRECT_URL=http://localhost:5173/auth/callback
VELORA_CASDOOR_PASSWORD_LOGIN_ENABLED=true
VELORA_CASDOOR_APPLICATION=<Casdoor application name>
VELORA_CASDOOR_ORGANIZATION=built-in
```

`VELORA_OIDC_INTERNAL_URL` 只用于容器内访问 Casdoor；对外 issuer 仍保持 Casdoor 的公开地址。生产环境必须使用 HTTPS、Secret Manager 和明确的密码代理风险接受；更高合规等级优先关闭该开关，改用 Authorization Code + PKCE，让密码和 MFA 只在 Casdoor 页面处理。

## 身份映射

Velora 不按邮箱或登录名自动绑定外部身份。Casdoor 用户的 `sub` 必须预先存在于 Velora 的 `external_identities(provider=casdoor)`，否则登录会被拒绝。管理员完成映射后再执行登录验收。

## 安全约束

- 登录请求仍受 IP + 账号双维限流和请求字段长度限制。
- 原始密码只存在于当前请求体和到 Casdoor 的内存请求中，不写日志、不写 Velora 数据库、不下发给前端以外的服务。
- Casdoor 的 MFA 错误不会降级到 Velora 本地密码。
- 关闭 `VELORA_CASDOOR_PASSWORD_LOGIN_ENABLED` 后，登录页自动回到标准 Casdoor OIDC 按钮。
