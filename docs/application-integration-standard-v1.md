# Velora 应用接入规范 V1

状态：强制执行（唯一权威清单，2026-08-23 修订）
适用范围：所有新接入 Velora 的生产应用  
当前样板：Spectra  
协议范围：OIDC Authorization Code + PKCE S256、账号与权限事件下发

本文定义“接入完成”的最低产品、协议、安全、运维与回滚标准。详细原理见[OIDC 接入指南](./application-oidc-integration-guide.md)，账号事件结构见[统一账号与应用接入标准](./account-provisioning-and-application-onboarding.md)。两份文档与本文冲突时，以本文为准。

## 1. 产品与数据边界

| 系统 | 必须负责 | 禁止负责 |
|---|---|---|
| Velora | 用户生命周期、应用目录、应用发布、访问策略、应用角色授权、下发状态、管理审计 | 保存密码、下游 Client Secret、应用业务权限判定、签发 OIDC Token |
| Casdoor | 密码与 MFA、浏览器 SSO、OIDC Client、Code/Token/JWKS/UserInfo | 面向普通用户提供独立入口、管理下游业务角色、替代 Velora 应用授权 |
| 下游应用 | OIDC RP 校验、本地业务 Session、业务 RBAC/数据权限、接收账号与角色事件、应用审计 | JIT 默认建号、默认授予角色、读取 Velora/Casdoor Cookie、接收用户密码或 MFA Secret |

普通用户只看到 Velora 登录页。`auth.sevoniva.com` 是协议域，不是产品入口；根路径、Casdoor SPA、账号页和管理 API 默认 404。Casdoor 源码不修改，Velora 不建设第二个 OIDC Provider。

## 2. 十五分钟标准接入路径

1. 应用团队提交不可变应用编码、HTTPS 首页、精确 Callback、登出回跳、负责人和角色目录。
2. 身份管理员创建独立 OIDC Client；只启用 Authorization Code、`openid profile email`、Signin Session，Callback 不得使用通配符。
3. Client Secret 一次性写入应用 Secret Manager/只读文件；禁止进入 Velora、Git、镜像、前端或工单正文。
4. Velora 创建应用和身份绑定，Issuer 必须精确为 `https://auth.sevoniva.com`，保存公开 Client ID 与 Callback 清单。
5. 应用实现服务端 OIDC、服务端 Session、账号下发接收端和业务权限默认拒绝。
6. 先部署应用接收端，再启用 Velora 下发；用专用测试账号执行本文第 10 节全链路验收。
7. Velora 验证身份绑定、配置允许范围、审批发布；记录 commit、镜像摘要、配置摘要和回滚点。

任一步缺失都只能标记 `DRAFT` 或 `VERIFICATION_PENDING`，不得显示为“已接入”。

## 3. OIDC 强制配置

```text
Issuer=https://auth.sevoniva.com
Discovery=https://auth.sevoniva.com/.well-known/openid-configuration
Flow=authorization_code
PKCE=S256
Scopes=openid profile email
RedirectURI=https://<application-domain>/<exact-callback>
```

应用必须自行生成并在服务端单次事务中保存：

- 至少 256 bit 随机 `state`；
- 至少 256 bit 随机 `nonce`；
- PKCE verifier，以及它的 S256 challenge；
- 创建时间、五分钟以内的过期时间和安全站内回跳路径。

事务 Cookie 必须为 Host-only、Secure、HttpOnly、SameSite=Lax，回调后立即删除；事务只能消费一次。应用不能让 Velora 为自己生成 state、nonce 或 verifier。

Velora 的跨域会话桥接还会在 `auth` 域设置一次性浏览器 nonce，并把摘要绑定到 30 秒票据。票据必须同时满足可信 `Origin`、非 cross-site Fetch Metadata、原浏览器 nonce 和原子单次消费；应用不得读取、转发或自行模拟该票据。

## 4. Callback 与 Token 校验

Callback 必须依次执行，任一步失败即拒绝：

1. 校验浏览器事务 Cookie 与 Query `state` 恒定时间相等并原子消费；
2. 拒绝 IdP `error`、空 Code、过期或重放事务；
3. 携带原 PKCE verifier 从服务端交换 Code；
4. 使用 Discovery/JWKS 验证 ID Token 签名；
5. `iss` 精确等于生产 Issuer；
6. `aud` 包含当前 Client ID，`exp` 有效，允许时钟偏差不得超过批准值；
7. Token `nonce` 与事务一致；
8. UserInfo `sub` 与 ID Token `sub` 一致；
9. 只允许已预配且状态为 `ACTIVE` 的本地账号建立业务 Session。

`sub + iss` 是跨系统稳定身份键；登录名、邮箱和显示名均可变化，不得作为关联主键。应用更换 Issuer 或 Client 必须重新验证和发布，不能静默迁移 subject。

## 5. 应用本地 Session

- OIDC Token 只在应用后端交换与验证，不写 LocalStorage、SessionStorage、URL 或日志。
- 应用签发自己的随机服务端 Session；Cookie 使用 Host-only、Secure、HttpOnly、SameSite=Lax、Path `/`。
- Session 绑定本地用户、组织、认证来源、创建/最后活动/绝对过期时间和撤销状态。
- 每次请求重新检查用户为启用状态；停用事件必须撤销全部 Session 和个人 API Token。
- 登录、Callback、退出响应使用 `Cache-Control: no-store`。

## 6. 默认登录与应急本地登录

- 应用 `/login` 必须直接发起 Velora SSO，并只接受经过校验的站内 `redirect`。
- 未认证访问业务页面必须跳 `/login?redirect=<safe-relative-path>`。
- 不得在普通登录页展示 Casdoor URL、Client ID、技术错误或账号密码表单。
- 若业务必须保留 break-glass，本地账号密码页只能位于不导航展示的 `/normal-login` 等专用路径。
- 专用本地登录仍必须执行短时随机登录 CSRF、同站 Cookie、IP 限流、失败锁定、强密码、TOTP/恢复码和独立审计；隐藏 URL 本身不是安全控制。
- 普通用户由 Velora/Casdoor 统一账号体系管理；本地账号仅用于批准的应急操作，不能承接日常用户。

## 7. MFA 规范

V1 暂不强制全部用户 MFA，但应用和门户必须允许用户自助开启、验证、生成恢复码和关闭 TOTP。管理员或敏感操作的强制策略可单独开启。

MFA 完成状态只能来自已验证的签名 Claims（如 AMR/ACR）或本系统实际完成的 TOTP/WebAuthn 校验。请求中出现非空 `mfa_code`、`recovery_code` 或前端布尔值绝不是 MFA 证明。认证来源与认证强度必须分开保存，不能因完成 MFA 丢失 `FEDERATED` 来源。

## 8. 账号与角色下发

应用必须实现 `POST /api/v1/provisioning/events`，使用每应用独立 HMAC Secret：

```text
X-Velora-Timestamp: <Unix seconds>
X-Velora-Signature: v1=<HMAC-SHA256(secret, timestamp + "." + raw-body)>
```

接收端必须限制 Body 64 KiB、严格 JSON、五分钟时钟偏差、恒定时间验签，并满足：

- `event_id` 全局幂等；重复返回 `DUPLICATE`；
- `aggregate_version` 单调；旧版本返回 `STALE`；
- 应用编码和角色必须在本地白名单；未知值拒绝；
- `ACTIVE` 只替换 `source=VELORA` 的角色；
- `DISABLED` 必须为空角色并原子完成停用、撤销 Session/API Token、保存事件和审计；
- 不接收密码、MFA Secret、Cookie、OIDC Token 或 Client Secret。

Velora 负责平台角色与应用角色的映射。下游应用仍负责把收到的应用角色映射到业务 permission/data scope；未知或无角色用户可以合法登录但只能看到无权限空态，不能报“网络异常”，也不能得到默认 `developer/admin`。

## 9. 退出与撤权

应用退出顺序固定为：

1. 撤销应用本地 Session 并清 Cookie；
2. 若认证来源为 OIDC，跳转受信任的 `https://auth.sevoniva.com/_velora/logout`；
3. 身份域清理 Casdoor 和 Velora gateway 会话；
4. 回到 Velora 登录入口或已登记的安全回跳。

退出 URL 必须来自受信任配置，并限制为 Issuer 主机和批准路径；不得接受任意 Query URL。普通 OIDC、OIDC+MFA 都必须走联邦退出。未实现并验证 Back-Channel Logout 前，不得宣称一个应用退出会主动撤销其他应用已建立的本地 Session；用户停用事件必须作为立即撤权通道。

## 10. 发布验收矩阵

| 场景 | 必须结果 |
|---|---|
| Discovery/JWKS | Issuer 与端点均为 `auth.sevoniva.com`，TLS 有效 |
| 正常登录 | Code + PKCE、state、nonce 全部通过并建立应用 Session |
| 默认入口 | `/login` 进入 Velora，用户看不到 Casdoor UI |
| 站内回跳 | 登录后回原业务路径；`//host`、绝对 URL、反斜杠和 Fragment 被拒绝 |
| Callback 攻击 | 错 state、nonce、aud、iss、过期 Token、重放 Code 全部拒绝 |
| 会话桥接 | 浏览器 A 生成的票据在浏览器 B 拒绝；跨站表单拒绝；票据不进入 URL/日志 |
| 无账号 | OIDC 身份有效但未预配时拒绝应用 Session |
| 无权限 | 返回空态/403，不报网络异常、不默认授权 |
| 角色变化 | 更高版本事件生效，旧版本不能覆盖 |
| 停用 | 应用 Session/API Token 撤销，后续登录拒绝 |
| 重试 | 同一事件得到 `DUPLICATE`，不产生重复副作用 |
| 退出 | 应用 Session、gateway、Casdoor 会话清理；不展示 JSON 响应页 |
| 本地应急 | 仅专用路径可见；CSRF、限流、锁定、MFA、审计有效 |
| Secret | Git、镜像、前端、日志和验收文档均无 Secret/Token/Cookie |
| 审计 | 登录成功/失败、下发、角色变化、停用、退出和管理发布可追踪 |

测试必须同时覆盖允许与拒绝场景。Mock 只能作为单元测试；生产发布必须使用真实 Issuer、真实 Client、真实浏览器和真实测试账号。

## 11. 稳定错误与用户文案

应用应保留稳定内部错误码并向用户显示可行动文案：

| 类别 | 用户提示 | 运维动作 |
|---|---|---|
| OIDC 暂不可用 | “统一登录暂时不可用，请稍后重试” | 检查 Discovery、证书、gateway、Casdoor |
| 登录事务失效 | “登录请求已失效，请重新登录” | 记录 request ID，不回显 state/code |
| 未开通 | “账号尚未开通此应用，请联系管理员” | 检查预配事件和 entitlement |
| 无业务权限 | 正常空态并说明申请方式 | 检查角色映射，不返回 5xx |
| 账号停用 | “账号已停用，请联系管理员” | 检查 Velora/Casdoor/应用状态一致性 |
| 下发失败 | 管理端显示重试状态 | 保留可靠事件并告警，禁止伪成功 |

日志只记录 request ID、内部用户 ID、应用编码、事件 ID、版本和稳定错误码；不记录敏感协议载荷。

## 12. 配置、发布与回滚

最低配置：

```text
OIDC_ISSUER=https://auth.sevoniva.com
OIDC_CLIENT_ID=<public-client-id>
OIDC_CLIENT_SECRET_FILE=/run/secrets/<application>-oidc-client-secret
OIDC_REDIRECT_URL=https://<application-domain>/<exact-callback>
OIDC_LOGOUT_URL=https://auth.sevoniva.com/_velora/logout
PROVISIONING_SECRET_FILE=/run/secrets/<application>-provisioning-secret
```

发布前：创建数据库/制品/配置回滚点，校验 Secret 可读权限，执行迁移，部署接收端，再启用下发和入口。发布后：检查 commit/镜像一致性、健康、登录、无权限、停用、退出和审计。

回滚时先在 Velora 下架入口并暂停对应下发，恢复上一制品和配置；additive migration 与账号/事件/审计数据保留，不以删除表或覆盖生产数据库作为普通回滚方式。必要时停用 OIDC Client，防止旧版本继续接受登录。

## 13. Spectra 样板映射

| 标准能力 | Spectra 实现 |
|---|---|
| 默认 SSO | `/login` → `/api/v1/auth/oidc/login` |
| 受控本地登录 | `/normal-login`，一次性登录 CSRF + 限流 + 锁定 + TOTP/恢复码 + 审计 |
| OIDC Callback | `/api/v1/auth/oidc/callback`，state/nonce/PKCE/ID Token/UserInfo 校验 |
| 安全回跳 | OIDC 服务端单次事务保存校验后的相对路径 |
| 账号下发 | `/api/v1/provisioning/events`，HMAC、幂等、版本、事务与审计 |
| MFA 自助 | 账户安全页开启、校验、恢复码再生成、关闭 |
| 联邦退出 | 本地撤销后跳受信任的 Velora logout URL |
| SecretStore | `SPECTRA_MASTER_KEY_FILE` 只读 Secret 文件，禁止写入环境变量、镜像或 Git |

代码验收基线：Velora `main` 的 `05f1216`，Spectra `main` 的 `c5e3c40`。自动化测试、部署状态和仍需人工完成的真实浏览器验收见[生产 V1 收口记录](./production-v1-closure-2026-08-23.md)。

## 14. 每个接入应用必须交付

- 应用登记表：编码、负责人、生产域名、精确 Callback、登出回跳、角色目录和数据分级；
- 配置清单：Issuer、Client ID、Secret 文件路径、Provisioning URL 与 Secret 文件路径；
- 自动化证据：state/nonce/PKCE、错误 Callback、幂等/旧版本、停用撤权、无权限空态和安全回跳；
- 生产证据：真实浏览器登录/退出、commit、镜像摘要、配置摘要、健康检查和审计事件；
- 回滚包：上一制品、上一配置、数据库备份、应用下架与 Client 停用步骤；
- 运维归属：告警接收人、Secret/证书轮换人、故障升级路径和维护窗口。

缺少任一项，应用状态只能是 `DRAFT` 或 `VERIFICATION_PENDING`，不能标记为 `PUBLISHED`。
