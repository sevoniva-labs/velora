# Velora 应用接入规范 V1

状态：唯一权威规范（2026-08-24，Spectra 与 Reference App 生产验证通过）
排障：[application-integration-troubleshooting.md](./application-integration-troubleshooting.md)

## 1. 产品边界

Velora 是管理员和用户的唯一入口，负责应用目录、访问范围、账号生命周期、角色、审批、验证证据和发布。Casdoor 是隐藏的认证协议引擎，负责密码、MFA、OIDC Client、Token 和统一会话；标准流程不打开 Casdoor。目标应用负责 OIDC RP、本地 Session、账号同步接收端和最终业务鉴权。

Velora 不自建 OIDC Provider，不修改 Casdoor，不向下游发送密码、MFA Secret、Cookie 或 Token。Client Secret 不进入业务数据库；Provisioning Secret 使用 KEK 信封加密。新增应用不得修改 Velora 代码、Worker 分支或增加应用专属环境变量。

## 2. 管理员流程

入口：`管理后台 → 应用管理 → 新建应用/接入配置`。

1. 填写名称、不可变编码、负责人、分类和生产地址。
2. OIDC 应用填写精确 HTTPS Callback；Issuer、Scopes、Client ID、PKCE 和 Provider Ref 由 Velora 生成。
3. 填写账号同步 HTTPS Endpoint、角色目录和访问范围。无策略默认拒绝；全员必须显式选择 `EVERYONE`。
4. 点击“申请审批并生成接入配置”。Velora 自动选择独立安全审批人并创建待办，不输入审批 ID。
5. 审批通过后再次点击，Velora 自动编排 Casdoor，返回五分钟、单次消费的 Enrollment Token。
6. 在应用服务器用 `velora-connect` 领取并部署，运行全部自动检查后试运行、发布。

状态以服务端 `status / next_action / blockers` 为准：`DRAFT → APPROVAL_PENDING → CREDENTIALS_ISSUED → WAITING_FOR_DEPLOYMENT → VERIFIED → PILOT → PUBLISHED`；异常为 `ACTION_REQUIRED / DEGRADED / SUSPENDED`。

凭据申请、审批和执行属于高风险操作，必须使用交互式会话和近期 MFA；API Token 不能代替人员审批。执行中断或 Token 过期时重新签发会轮换 Client Secret，旧值立即退出使用。提交发布对已处于 `READY` 的同版本配置幂等，不会使刚通过的版本化检查失效。

## 3. 安全领取

从 release 获取并核对 `SHA256SUMS`：

```bash
install -d -m 0700 /etc/<application>/velora
velora-connect enroll --portal https://home.sevoniva.com --output /etc/<application>/velora
velora-connect doctor --config /etc/<application>/velora/velora.env
```

Token 从标准输入或权限 `0600` 的 `--token-file` 读取，不放入命令参数、Shell History、工单或聊天。CLI 原子写入 `velora.env`、`oidc-client-secret`、`provisioning-secret`，权限均为 `0600`。领取即销毁；丢失时轮换，不能查询旧值。

## 4. OIDC 契约

必须使用 Authorization Code + PKCE S256，Scopes 为 `openid profile email`，Callback 为精确 HTTPS，禁止通配符、Query、Fragment 和 userinfo。应用 Session 使用 Host-only、Secure、HttpOnly、SameSite=Lax Cookie。

Go SDK：`github.com/sevoniva-labs/velora/server/sdk/velora`。

```go
client, err := velora.NewOIDCClient(ctx, velora.OIDCConfig{
    Issuer: issuer, ClientID: clientID, ClientSecret: secret, RedirectURL: callback,
})
authorization, err := client.NewAuthorization()
// State、Nonce、PKCEVerifier、ExpiresAt 保存到服务端单次事务存储。
identity, err := client.Exchange(ctx, code, authorization.Nonce, authorization.PKCEVerifier)
```

SDK 校验 Discovery/JWKS、Issuer、Audience、Expiry、Nonce 和 PKCE；应用负责将 State 绑定浏览器会话、恒定时间比较并单次删除。默认登录跳 Velora；`/normal-login` 仅作受控 break-glass。

浏览器可能先请求公网身份域名的标准授权端点，但未建立企业会话时，身份网关必须立即 302 到 `https://home.sevoniva.com/login?app=<code>...`；页面、文案和普通导航不得出现 Casdoor。目标应用不得把 Casdoor 管理地址或内部容器地址返回浏览器。

## 5. 账号同步契约

```text
POST <Provisioning Endpoint>
X-Velora-Timestamp: <Unix seconds>
X-Velora-Signature: v1=<hex HMAC-SHA256(secret, timestamp + "." + raw-body)>
X-Request-ID: <event id>
```

```go
handler, err := velora.NewProvisioningHandler(secret, productionTransactionalStore)
mux.Handle("/api/v1/integrations/velora/provisioning", handler)
```

`ProvisioningStore.Apply` 必须在同一数据库事务完成 event_id 幂等、聚合版本、用户投影、角色替换、停用和 Session/Token 撤销，只返回 `APPLIED / DUPLICATE / STALE`。未知角色、未预配用户和未知状态失败关闭；`DISABLED` 的角色必须为空。SDK 强制 64 KiB、严格 JSON、五分钟时钟偏差、恒定时间 HMAC 和 `integration.challenge`，不替应用实现 RBAC 或持久化事务。

## 6. 发布门禁

当前配置版本必须通过：显式访问范围、OIDC Discovery/Issuer/JWKS、签名 challenge=`APPLIED`、相同事件=`DUPLICATE`、低版本事件=`STALE`。配置变化使旧证据失效；后端强制门禁，不能绕过前端发布。

人工验收还需确认：真实浏览器登录及原路径回跳、无角色空态、退出回到 Velora且不暴露 Casdoor、停用撤销应用 Session、恢复后显式重新授权。

## 7. 样板、发布与回滚

`server/cmd/oidc-demo` 是最小 Reference App，使用 SDK Provisioning Handler；Spectra 是复杂样板，但不是特殊分支。Reference App 账号同步路径为 `/api/v1/integrations/velora/provisioning`，配置增加 `DEMO_PROVISIONING_SECRET_FILE`。

发布顺序：备份与配置摘要 → additive migration → Server/Web/Worker → 目标应用 → 自动检查 → 试运行 → 人工登录/退出 → 发布。监控 Reliable Message 积压/失败、Target `DEGRADED`、签名/时钟、OIDC、审批和 Enrollment；日志不得含 Secret/Token/Cookie/授权码。

回滚先暂停入口和新下发，再恢复上一镜像；事故窗口不删除 additive 表列。已签发 Secret 不随镜像回滚，必须轮换；Casdoor 自动化失败时保留既有 Client。

当前生产证据见 [application-onboarding-production-evidence-2026-08-24.md](./application-onboarding-production-evidence-2026-08-24.md)。

## 8. 验收记录

```text
Velora/Application commit+image:
Config version / test account:
Enrollment without copied Secret: PASS/FAIL
OIDC Discovery + real login + original path: PASS/FAIL
Provision APPLIED/DUPLICATE/STALE: PASS/FAIL
No-role empty state: PASS/FAIL
Disable/session revoke/restore/logout: PASS/FAIL
Secret absent from DB/log/audit/idempotency replay: PASS/FAIL
Rollback point / executed at/by:
```
