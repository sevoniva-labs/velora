# Velora 应用接入手册

> 微信扫码属于门户登录能力，不改变业务应用的 OIDC 与账号下发接入方式。身份平台侧申请与配置见[微信扫码登录接入与运维](./wechat-login.md)。

状态：唯一权威接入文档  
适用对象：应用申请人、应用负责人、开发、运维、平台管理员、安全审批人  
生产入口：`https://home.sevoniva.com`  
统一身份协议地址：`https://auth.sevoniva.com`

## 1. 先判断需要接入什么

| 模式 | 能力 | 适用场景 | 是否推荐 |
|---|---|---|---|
| 门户链接 | 应用展示、访问范围和启动 | 已有独立登录且暂不统一账号 | 仅过渡 |
| 统一登录 | OIDC SSO、应用本地 Session | 应用已有自己的账号管理 | 可用 |
| 统一登录 + 账号下发 | SSO、实时开通/改权/停用 | 大多数企业业务系统 | 推荐 |
| 统一登录 + 账号下发 + 目录读取 | 实时推送、首次全量、对账和恢复 | 需要组织树、批量同步或严格对账 | 完整模式 |

新生产应用默认采用完整模式。只接 OIDC 不等于完成账号治理；只有登录、授权、停用和回滚全部通过，才允许发布。

## 2. 谁负责什么

| 角色 | 必须完成的事项 |
|---|---|
| 应用申请人 | 说明业务用途、用户范围、数据需求和负责人 |
| 应用负责人 | 确认域名、角色目录、上线窗口、验收人和回滚负责人 |
| 应用开发 | 实现 OIDC RP、本地 Session、账号下发接收端和业务权限 |
| 应用运维 | 保存 Secret、部署配置、提供健康接口、监控和回滚 |
| 平台管理员 | 建立应用、配置使用范围、发起凭据审批、检查和发布 |
| 审批人 | 审批生产凭据、高权限角色、大范围授权和发布 |
| 身份管理员 | 仅处理自动化失败和应急问题；标准流程不登录 Casdoor |

## 3. 申请接入前，应用团队要给平台什么

提交以下信息。缺一项时应用保持草稿，不签发生产凭据。

### 3.1 基本信息

| 字段 | 要求 | 示例 |
|---|---|---|
| 应用名称 | 面向用户的正式名称 | 订单中心 |
| 应用编码 | 小写字母、数字和短横线；创建后不可变 | `order-center` |
| 业务用途 | 一句话说明解决什么问题 | 处理企业订单与退款 |
| 应用负责人 | 对接与上线最终责任人 | 张三 |
| 负责部门 | 负责人所在部门 | 交易平台部 |
| 生产首页 | 公网或企业网络内 HTTPS 地址 | `https://order.example.com` |
| 帮助/故障联系人 | 用户无法访问时的联系渠道 | 工单队列或值班组 |

### 3.2 登录信息

每个环境提供精确 Callback：

```text
开发：https://dev-order.example.com/api/auth/oidc/callback
测试：https://test-order.example.com/api/auth/oidc/callback
生产：https://order.example.com/api/auth/oidc/callback
退出后回跳：https://order.example.com/login
```

生产和测试必须是 HTTPS。只允许开发机使用 `http://localhost` 或 `http://127.0.0.1`。禁止通配符、Query、Fragment、用户名密码和模糊前缀匹配。

应用团队同时确认：

- 使用 Authorization Code Flow；
- 支持 PKCE S256；
- 能校验 Discovery、JWKS、Issuer、Audience、Nonce 和 Expiry；
- Session 存在服务端，不把 Token 写入 LocalStorage；
- 默认登录入口跳统一登录，特殊 `/normal-login` 仅作为受控应急入口。

### 3.3 账号下发信息

| 字段 | 要求 | 示例 |
|---|---|---|
| 接收地址 | 生产 HTTPS POST 接口 | `https://order.example.com/api/v1/integrations/velora/events` |
| 事务存储 | 支持事件幂等与版本比较 | PostgreSQL |
| Session 撤销 | 收到停用后立即失效 | 已实现 |
| 最大响应时间 | 建议 3 秒内 | 1 秒 |
| 健康地址 | 可从平台运行环境访问 | `/healthz`、`/readyz` |

### 3.4 应用角色

应用角色必须是业务角色，不是 Velora 平台管理员角色。

| 角色编码 | 名称 | 能做什么 | 风险级别 |
|---|---|---|---|
| `viewer` | 查看者 | 查看订单 | 普通 |
| `operator` | 操作员 | 创建和修改订单 | 普通 |
| `approver` | 审批员 | 审批退款 | 高权限 |
| `application_admin` | 应用管理员 | 管理应用内配置 | 关键权限 |

每个角色必须说明权限，不接受 `admin1`、`other`、`test` 等含义不明的编码。高权限角色需要审批和定期复核。

### 3.5 初始使用范围

选择一种或多种：

- 部门，可选择是否包含下级部门；
- 用户组；
- Velora 平台角色；
- 指定人员；
- 全体成员，必须显式选择；
- 排除规则，优先级高于允许规则。

没有任何允许规则时默认拒绝，不能为了联调临时全员放开。

### 3.6 上线信息

- 计划上线时间和变更窗口；
- 应用提交、镜像摘要和数据库迁移版本；
- 监控、告警和当班负责人；
- 回滚镜像、回滚配置和数据库兼容说明；
- 测试账号及期望角色；
- 数据字段需求及必要性说明。

## 4. 平台会给应用团队什么

| 交付物 | 何时提供 | 用途 | 是否敏感 |
|---|---|---|---|
| Application ID / Code | 创建草稿后 | 接口路径、审计和配置标识 | 否 |
| Issuer / Discovery | 创建草稿后 | OIDC 发现与 Token 校验 | 否 |
| Client ID | 凭据审批后 | OIDC Audience | 否 |
| Client Secret | 凭据审批后，仅交付一次 | 服务端换取 Token | 是 |
| Provisioning Secret | 配置账号下发后，仅交付一次 | 验证平台推送签名 | 是 |
| Directory Token | 同一接入包中，仅交付一次 | 读取组织、部门和授权用户 | 是 |
| 接入配置文件 | 领取接入包后 | 应用运行时配置 | 含 Secret 路径，不含明文 |
| 上线检查结果 | 联调后 | 判断是否允许发布 | 否 |

真实 Secret 不通过聊天、邮件、工单或普通下载包发送。平台只显示五分钟有效、单次消费的 Enrollment Token；应用运维在目标服务器运行 `velora-connect` 领取。

## 5. 从申请到上线的完整旅程

### 第一步：提交接入申请

应用团队按第 3 节提交资料。平台管理员进入：

```text
平台管理 → 应用中心 → 应用 → 新建应用
```

填写应用名称、编码、负责人、部门、生产地址、分类和登录方式。创建后进入应用详情。

### 第二步：配置应用角色和使用范围

在“应用角色”维护业务角色；在“使用范围”选择部门、用户组、平台角色或人员，并把应用角色分配给这些主体。保存前页面会预览新增、撤销、角色变化、高权限用户和账号下发任务数量。

### 第三步：配置账号下发（需要同步账号时）

在“账号下发”填写应用接收地址。平台生成独立 Provisioning Secret。每个应用使用自己的 Secret，禁止复制其他应用配置。

首次接入且尚未配置登录时，页面不暴露密钥，后续通过接入包统一交付。登录已配置后的新增或轮换会一次性显示 Provisioning Secret、Directory Token 和目录接口路径。账号下发密钥轮换时 Directory Token 也会同步更换，应用团队必须把两项新密钥一起更新。

如果应用只需要统一登录，可跳过本步骤。需要账号下发或目录读取时，应先完成本步骤，再申请登录配置，使一次性接入包同时包含全部凭据。

### 第四步：申请统一登录配置

在“登录设置”录入各环境 Callback。平台自动生成标准 Issuer、Scopes、Client ID 和 Casdoor 应用配置。普通管理员不需要登录 Casdoor，也不填写 Provider Ref、Grant Type 或 PKCE 参数。

凭据签发需要审批。等待审批时不重复提交；审批通过后重新保存，平台生成接入包。未配置账号下发时，接入包只包含 OIDC 配置，这是有效的“仅统一登录”模式。

### 第五步：在应用服务器领取接入包

从正式发布制品获取与服务器架构匹配的 `velora-connect`，先核对 `SHA256SUMS`，然后执行：

```bash
install -d -m 0700 /etc/order-center/velora
velora-connect enroll \
  --portal https://home.sevoniva.com \
  --output /etc/order-center/velora

velora-connect doctor \
  --config /etc/order-center/velora/velora.env
```

Enrollment Token 从标准输入读取，或使用权限 `0600` 的 `--token-file`。不要把 Token 写在命令参数中。

CLI 必定生成：

```text
velora.env                 0600
oidc-client-secret         0600
```

配置账号下发后还会生成：

```text
provisioning-secret        0600
directory-token            0600
```

`velora.env` 必定包含：

```text
VELORA_APPLICATION_ID
VELORA_APPLICATION_CODE
VELORA_OIDC_ISSUER
VELORA_OIDC_CLIENT_ID
VELORA_OIDC_CLIENT_SECRET_FILE
VELORA_OIDC_REDIRECT_URI
VELORA_OIDC_SCOPES
```

完整模式还包含：

```text
VELORA_PROVISIONING_ENDPOINT
VELORA_PROVISIONING_SECRET_FILE
VELORA_PROVISIONING_KEY_VERSION
VELORA_PROVISIONING_FINGERPRINT
VELORA_DIRECTORY_BASE_URL
VELORA_DIRECTORY_TOKEN_FILE
```

### 第六步：联调和上线检查

平台管理员进入“上线检查”，依次完成：

1. OIDC Discovery 和 Client 配置检查；
2. Callback、Issuer、Client ID、Scopes 和 PKCE 检查；
3. Provisioning Challenge；
4. 重复事件返回 `DUPLICATE`；
5. 低版本事件返回 `STALE`；
6. 访问策略和测试账号检查；
7. 真实浏览器登录、退出和无权限拒绝；
8. 目录接口全量、增量和错误凭据检查。

全部通过后提交上线审批。批准后确认上线；未通过时应用保持不可用，不允许绕过检查直接发布。

## 6. OIDC 实现要求

### 6.1 固定协议参数

```text
Issuer: https://auth.sevoniva.com
Discovery: https://auth.sevoniva.com/.well-known/openid-configuration
Flow: authorization_code
PKCE: S256
Scopes: openid profile email
```

Issuer、Client ID、Client Secret 和 Callback 以 `velora.env` 为准，不手工猜测。

### 6.2 登录事务

应用发起登录时：

1. 生成至少 32 字节随机 State；
2. 生成至少 32 字节随机 Nonce；
3. 生成 PKCE Verifier 和 S256 Challenge；
4. 将 State、Nonce、Verifier、目标站内路径和过期时间存入服务端单次事务存储；
5. 浏览器跳转 Discovery 返回的 Authorization Endpoint；
6. Callback 恒定时间比较 State，单次删除事务；
7. 使用 Code、Verifier 和 Client Secret 换 Token；
8. 校验签名、Issuer、Audience、Expiry 和 Nonce；
9. 查询本地已下发账号，只有 `ACTIVE` 才建立 Session。

State 不能只存在浏览器可修改参数中，Code 不能重复交换，Callback 不能接受站外回跳地址。

### 6.3 Go SDK

模块：`github.com/sevoniva-labs/velora/server/sdk/velora`。

```go
client, err := velora.NewOIDCClient(ctx, velora.OIDCConfig{
    Issuer:       issuer,
    ClientID:     clientID,
    ClientSecret: clientSecret,
    RedirectURL:  callback,
})
if err != nil { return err }

authorization, err := client.NewAuthorization()
if err != nil { return err }
// 将 State、Nonce、PKCEVerifier、ExpiresAt 保存到服务端单次事务存储。

identity, err := client.Exchange(
    ctx,
    code,
    authorization.Nonce,
    authorization.PKCEVerifier,
)
```

应用仍负责 State 与浏览器会话绑定、事务单次消费和站内回跳验证。

### 6.4 Session

- Cookie 推荐 `__Host-<app>_session`；
- `Secure`、`HttpOnly`、`SameSite=Lax`、`Path=/`；
- 登录、Callback 和退出响应使用 `Cache-Control: no-store`；
- Session ID 不放 URL；
- 用户停用、角色撤销或管理员强制退出时立即撤销；
- 应用退出先删除自己的 Session，再进入统一退出流程；
- 未实现 Back-Channel Logout 前，不宣称退出一个应用会自动清理全部应用。

## 7. 账号下发接口

### 7.1 HTTP 契约

应用提供：

```text
POST https://<application>/api/v1/integrations/velora/events
Content-Type: application/json
X-Velora-Timestamp: <Unix seconds>
X-Velora-Signature: v1=<hex HMAC-SHA256(secret, timestamp + "." + raw-body)>
X-Request-ID: <event id>
```

Body 最大 64 KiB，时钟偏差最大 5 分钟。必须对原始 Body 验签后再解析 JSON。

### 7.2 用户事件

```json
{
  "schema_version": "1.0",
  "event_id": "f7deccf3-f06f-4aa7-9801-2f315d41df52",
  "event_type": "user.entitlements.changed",
  "aggregate_version": 8,
  "occurred_at": "2026-08-25T01:00:00Z",
  "source": "velora",
  "user": {
    "subject": "stable-identity-subject",
    "issuer": "https://auth.sevoniva.com",
    "login_name": "carson",
    "display_name": "Carson",
    "email": "carson@example.com",
    "status": "ACTIVE"
  },
  "entitlements": {
    "application_code": "order-center",
    "roles": ["operator"]
  }
}
```

停用事件的 `status` 为 `DISABLED` 且 `roles` 必须为空。接收端必须在同一事务中更新用户、角色、版本、事件记录并撤销 Session/Token。

### 7.3 幂等和版本

接收端返回：

```json
{"status":"APPLIED"}
{"status":"DUPLICATE"}
{"status":"STALE"}
```

- `event_id` 相同且 Body 相同：`DUPLICATE`；
- `event_id` 相同但 Body 不同：拒绝；
- `aggregate_version` 小于当前版本：`STALE`；
- 未知应用、未知角色、错误签名、超时或非法字段：拒绝；
- 5xx 会触发平台重试，明确 4xx 进入配置故障处理。

生产接收端可以直接使用 Go SDK 的 `NewProvisioningHandler`，但必须实现事务型 `ProvisioningStore`；内存 Store 只用于测试。

## 8. 组织和用户目录接口

目录接口用于首次全量、定期对账和推送故障恢复，不代替实时账号下发。

### 8.1 认证和数据范围

```http
Authorization: Bearer <directory-token 文件内容>
Accept: application/json
```

Directory Token 与 Application ID 绑定，由 Provisioning Secret 派生但单独交付。它只能访问当前应用目录接口：

- 不接受 Velora 用户 Session；
- 不接受平台管理员 Token；
- 不能读取其他应用用户；
- 应用停用或账号下发密钥轮换后立即失效；
- 返回头包含 `Cache-Control: no-store`。

### 8.2 获取组织

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $(tr -d '\n' </etc/order-center/velora/directory-token)" \
  "$VELORA_DIRECTORY_BASE_URL/organization"
```

```json
{
  "code": "000000",
  "message": "success",
  "data": {
    "id": "organization-id",
    "key": "built-in",
    "name": "Sevoniva",
    "status": "ACTIVE",
    "updated_at": "2026-08-25T01:00:00Z"
  }
}
```

### 8.3 获取部门

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $(tr -d '\n' <.../directory-token)" \
  "$VELORA_DIRECTORY_BASE_URL/departments"
```

返回 `departments` 和 `snapshot_at`。部门包含 `id`、`parent_id`、`key`、`name`、`status`、`sort_order`、`updated_at`。应用按 `parent_id` 构建树，不根据名称猜测层级；停用部门仍返回，用于清理本地投影。

### 8.4 获取本应用已授权用户

首次全量：

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $(tr -d '\n' <.../directory-token)" \
  "$VELORA_DIRECTORY_BASE_URL/users?page_size=100"
```

增量同步：

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $(tr -d '\n' <.../directory-token)" \
  "$VELORA_DIRECTORY_BASE_URL/users?page_size=100&updated_after=2026-08-25T00:00:00Z"
```

```json
{
  "code": "000000",
  "message": "success",
  "data": {
    "users": [
      {
        "subject": "stable-identity-subject",
        "login_name": "carson",
        "display_name": "Carson",
        "email": "carson@example.com",
        "department_id": "department-id",
        "status": "ACTIVE",
        "roles": ["operator"],
        "version": "8",
        "updated_at": "2026-08-25T01:00:00Z"
      }
    ],
    "next_page_token": "opaque-token",
    "snapshot_at": "2026-08-25T01:05:00Z"
  }
}
```

分页规则：

1. `page_size` 默认 100，最大 500；
2. 有 `next_page_token` 时原样传入下一页，不解析、不修改；
3. 同一轮分页不要改变 `updated_after`；
4. 全部页完成后，把最后一轮 `snapshot_at` 保存为下一次 `updated_after`；
5. 只有本应用曾授权的用户会返回；撤权和停用用户以 `DISABLED` 返回且角色为空；
6. `subject` 是跨登录与下发链路的稳定主键，不使用登录名作为主键。

错误凭据、错误 Application ID、已停用应用统一返回 401 和 `DIRECTORY_UNAUTHENTICATED`，不泄露应用是否存在。

## 9. 应用本地数据模型建议

至少保存：

```text
velora_subject          唯一、不可变
issuer                  固定身份来源
login_name              可变展示/登录属性
display_name            可变
email                   可变
department_id           可空
status                  ACTIVE / DISABLED
provisioning_version    单调递增
velora_roles            仅保存平台下发的本应用角色
created_at / updated_at
```

本地特殊应急角色必须明确标记来源，不能被普通下发覆盖；业务权限计算应能区分 `source=VELORA` 和本地 break-glass。

管理员或用户修改显示名称、邮箱等应用必要属性后，Velora 会递增 `provisioning_version`，并向该用户已经授权的每个应用重新发送兼容的 `user.entitlements.changed` 1.0 事件；目录增量接口也会返回本次变更。接收方必须按 `subject` 幂等更新，且忽略低于本地版本的事件。真实姓名、手机、性别等敏感资料不会默认下发；确有业务必要时须单独申请属性释放范围和隐私审批，不能从管理接口旁路读取。

## 10. Secret 管理和轮换

- Secret 文件由应用运行用户读取，权限 `0400` 或 `0600`；
- 容器挂载后必须在容器内执行 `test -r`，不能只看宿主机权限；
- Kubernetes 使用 Secret/CSI/External Secrets，Compose 使用只读文件挂载；
- 日志只记录 Client ID 后四位、Provisioning 指纹、版本和 Request ID；
- 不打印 Authorization、Code、Cookie、Token 或 Secret；
- Client Secret 和 Provisioning Secret 分别轮换；Provisioning Secret 轮换会同步更换 Directory Token；
- 丢失 Secret 只能轮换，不能查询旧值；
- 应用停用后目录访问、账号下发和统一登录均应失败关闭。

## 11. 生产验收清单

### 配置

- [ ] 应用名称、编码、负责人、部门和生产地址正确。
- [ ] 开发、测试、生产 Callback 精确登记。
- [ ] 应用角色有明确说明和风险等级。
- [ ] 使用范围不是隐式全员。
- [ ] Secret 通过 `velora-connect` 写入受控文件。

### 登录和安全

- [ ] State、Nonce、PKCE S256 全部启用。
- [ ] 签名、Issuer、Audience、Expiry 和 Nonce 校验通过。
- [ ] 伪造 State、重放 Code、错误 Callback 和错误 Client Secret 被拒绝。
- [ ] 无本地预配账号和停用账号被拒绝。
- [ ] Session Cookie 属性正确，日志无敏感数据。
- [ ] 默认入口只出现 Velora 页面，不出现 Casdoor 登录或管理页面。

### 账号下发

- [ ] 新授权返回 `APPLIED` 并能登录。
- [ ] 相同事件返回 `DUPLICATE`。
- [ ] 低版本事件返回 `STALE`，不覆盖新权限。
- [ ] 停用事件清空角色并撤销 Session/Token。
- [ ] 错误签名、超时、大 Body、未知字段和未知角色被拒绝。
- [ ] 目标应用 5xx 后平台可重试并最终恢复。

### 目录接口

- [ ] 正确 Token 可读取组织和部门。
- [ ] 只返回当前应用已授权用户。
- [ ] 分页无重复、无遗漏。
- [ ] `updated_after` 增量同步正确。
- [ ] 错误 Token 和其他应用 Token 返回 401。
- [ ] 应用停用或密钥轮换后旧 Token 失效。

### 运维与回滚

- [ ] `/healthz`、`/readyz` 和监控可用。
- [ ] 应用提交、镜像摘要、配置摘要和数据库版本已记录。
- [ ] 回滚镜像、配置和数据库兼容路径已验证。
- [ ] 回滚后不会恢复旧 Secret 或降低账号版本。

## 12. 回滚顺序

1. 在 Velora 暂停应用入口或停用应用，阻止新增登录；
2. 停止应用侧消费和写操作；
3. 回滚应用镜像和配置，保留账号、事件和审计数据；
4. 必要时轮换 Client Secret 和 Provisioning Secret，不恢复旧明文；
5. 修复后使用更高版本重新下发，不手工降低 `aggregate_version`；
6. 重新执行登录、无权限、下发、目录和退出验收后恢复发布。

## 13. 常见问题

### `redirect_uri mismatch`

协议、域名、端口、路径或末尾斜杠与登记值不完全一致。复制应用实际 Callback，精确登记，不使用通配符。

### `issuer mismatch`

应用使用了内部容器地址、旧域名或带额外路径的地址。只使用接入包里的 Issuer，并确认 ID Token 的 `iss` 完全一致。

### `invalid_client`

Client ID/Secret 不匹配、Secret 已轮换或应用已停用。检查文件挂载和容器内可读权限，禁止打印 Secret。

### 登录后仍显示无权限

OIDC 只证明身份。检查账号下发是否 `APPLIED`、应用角色是否存在、用户状态是否 `ACTIVE`，不要用 JIT 建号绕过。

### 账号下发持续失败

检查 HTTPS 证书、DNS、目标应用健康、响应时间、签名时间差和事务错误。修复后在“账号下发”点击重新下发，不手工伪造事件。

### 目录接口返回 401

检查 Application ID、`directory-token` 文件、应用是否停用以及 Provisioning Secret 是否已轮换。目录 Token 与应用绑定，不能复制到另一个应用。

### 增量同步遗漏用户

必须完整消费当前分页，再把响应的 `snapshot_at` 作为下一轮 `updated_after`；不能使用应用本机当前时间代替服务器快照时间。

### Casdoor 页面或白屏出现在浏览器

这是身份网关或 Client 配置故障，不是正常产品路径。立即停止发布，检查统一入口、Session Bridge、Client 可选数组配置和反向代理；不要修改 Casdoor 前端掩盖脏配置。

## 14. 接入申请模板

```text
应用名称：
应用编码：
业务用途：
应用负责人 / 部门：
开发、测试、生产首页：
开发、测试、生产 Callback：
退出后回跳：
账号下发接收地址：
健康 / 就绪地址：
应用角色（编码、名称、说明、风险级别）：
初始使用范围：
需要目录接口：是/否；需要字段及理由：
计划上线窗口：
应用提交 / 镜像摘要 / 数据库版本：
测试账号和期望角色：
监控与告警负责人：
回滚负责人和回滚版本：
```

## 15. 平台交付确认模板

```text
Application ID / Code：
Issuer / Discovery：
Client ID：
Callback 登记摘要：
Enrollment Token 签发人 / 到期时间（不记录 Token）：
Provisioning Endpoint / Secret 指纹：
Directory Base URL：
应用角色版本：
使用范围版本：
上线检查结果：
真实登录 / 无权限 / 停用 / 退出结果：
应用提交 / 镜像摘要：
Velora 提交 / 镜像摘要：
回滚点：
验收人与时间：
```

任何字段无法确认时，不以口头说明代替，不进入生产发布。
