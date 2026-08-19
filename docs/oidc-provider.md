# 应用对接 Velora SSO（统一登录入口）

Velora 作为门户 + 统一登录入口：**所有登录都经 Velora**，应用的 SSO 跳转指向 Velora，
Casdoor 对应用完全不可见、永不修改。

## 一、标准 OIDC 应用（推荐）

在管理后台「应用管理」创建应用，接入类型选 **Velora SSO（VELORA_OIDC）**，
系统自动生成 OIDC 客户端。操作列「OIDC 客户端」可查看 / 新建 / 吊销客户端。

### 对接参数

| 参数 | 值 |
|---|---|
| issuer / 发现文档 | `https://<velora-host>/oidc/.well-known/openid-configuration` |
| authorization_endpoint | `https://<velora-host>/oidc/authorize` |
| token_endpoint | `https://<velora-host>/oidc/token` |
| userinfo_endpoint | `https://<velora-host>/oidc/userinfo` |
| jwks_uri | `https://<velora-host>/oidc/jwks` |
| 认证方式 | `client_secret_post` 或 `client_secret_basic` |
| PKCE | **强制 S256**（`code_challenge_method=S256`） |

### 回调（redirect_uri）白名单

客户端创建时提交的回调地址严格白名单（精确匹配、防开放重定向）；
需要新增回调时在「OIDC 客户端」重建客户端。

### 示例（通用 OIDC 客户端库）

```bash
# 1) authorize（用户浏览器）→ 拿 code
GET /oidc/authorize?client_id=<clientId>&redirect_uri=<callback>&response_type=code\
  &scope=openid%20profile%20email&state=<random>&code_challenge=<s256(verifier)>&code_challenge_method=S256

# 2) token 交换（服务端）→ 拿 access_token / refresh_token
POST /oidc/token
  grant_type=authorization_code&code=<code>&redirect_uri=<callback>&code_verifier=<verifier>
  Authorization: Basic base64(clientId:clientSecret)

# 3) userinfo → 用户身份
GET /oidc/userinfo  Authorization: Bearer <access_token>
```

- access_token：RS256 JWT（1 小时，含 `jti`、`sub`、`preferred_username`、`email`、`roles`、`groups`）。
- refresh_token：30 天，**每次刷新轮换**（旧 token 立即失效）；用 `grant_type=refresh_token` 换取新对。
- 密钥 90 天自动轮换（30 天宽限期内新旧签名均有效，见 JWKS）。

### 安全语义

- code 一次性、5 分钟有效、绑定 PKCE verifier 与 client；
- 吊销客户端（管理后台）→ 该客户端所有 code/token 立即失效；
- 用户会话吊销 / 改密 → 其全部 Velora 令牌一并失效。

## 二、非 OIDC 老系统（Forward Auth，网关 auth_request）

老系统无需改造 OIDC 库，在 Nginx/APISIX 配 `auth_request` 即可：

```nginx
# 接入类型选 FORWARD_AUTH，启动地址填老系统 URL（如 https://legacy.internal/dashboard）
location = /velora-forward-auth {
    internal;
    proxy_pass http://velora:8080/api/v1/forward-auth?next=$scheme://$host$request_uri;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
}

location / {
    auth_request /velora-forward-auth;
    auth_request_set $velora_user $upstream_http_x_velora_user; # 身份注入
    proxy_set_header X-Velora-User $velora_user;
    proxy_pass http://legacy_backend;
}
```

- 未登录 → Velora 返回 401 + `Location` 登录页（带 next 回跳），网关转跳；
- 已登录 → 200 + `X-Velora-User` / `X-Velora-Email` / `X-Velora-Role` 头，供上游识别身份。

## 三、对接检查清单

- [ ] 应用已创建且接入类型正确（VELORA_OIDC / FORWARD_AUTH）
- [ ] 回调地址加入客户端白名单
- [ ] 应用侧使用 PKCE S256（OIDC 库默认开启）
- [ ] 密钥已安全保存（仅创建时展示一次）
- [ ] 接入类型为 FORWARD_AUTH 的应用已配置网关 auth_request
- [ ] 联调：`/oidc/.well-known/openid-configuration` 可达；token 交换成功；userinfo 返回身份
