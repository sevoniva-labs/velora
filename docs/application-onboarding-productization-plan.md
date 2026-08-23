# Velora 应用接入产品化实施方案

状态：二次 Review 后待确认实施（2026-08-23）  
适用范围：Velora `main`、所有新接入应用  
目标：把 Spectra 专项接入改造成可复制、可自助、可审计、可回滚的通用产品能力

## 1. 决策结论

当前能力可以支撑一个应用通过专项开发接入，但不能支撑持续接入更多应用。问题不是 OIDC 本身，而是一个接入任务横跨 Velora、Casdoor、目标应用代码和服务器，且账号下发、角色目录、用户授权页面仍写死 Spectra。

本方案不修改 Casdoor，不建设 Velora 自有 OIDC Provider，不降低 Authorization Code + PKCE、Secret 隔离、默认拒绝和审批审计要求。改造方向是：

1. Velora 成为唯一应用接入控制台和配置事实来源。
2. Casdoor 继续作为隐藏的认证协议引擎，由 Velora 后端受控调用。
3. 目标应用只实现标准 OIDC RP、标准账号下发接收端和本地业务授权。
4. 新增应用不再修改 Velora 代码、增加应用专属配置字段或增加专属 Worker 主题。
5. 管理员通过一个向导完成创建、凭据生成、角色目录、访问策略、验证和发布。

在本方案的 Phase 0 至 Phase 4 核心项完成前，不建议宣称“产品化接入完成”。其中 Go SDK、Reference App、单一权威文档和安全接入 CLI 是便利性的必要组成，不再作为可省略增强项；其他语言 SDK 才是后续增强。

## 2. 已确认的现状差距

### 2.1 接入入口分裂

当前管理员先创建应用，再进入“身份与单点登录”选择应用；访问策略又跳转到其他页面。向导展示五个步骤，但只有身份绑定和发布动作位于同一上下文，不能形成连续任务。

### 2.2 协议参数暴露过多

管理员需要人工填写 `providerApplicationRef`、`publicClientId`、`issuer`、`scopes` 和 Redirect URI。生产环境中 Issuer、Scopes、授权模式和 PKCE 策略均为平台标准，不应成为普通表单字段。

### 2.3 Casdoor 自动化没有形成默认产品路径

后端已经具备受审批约束的 Casdoor Application Upsert、一次性 Client Secret 返回和停用能力，但生产示例仍默认关闭自动化。当前管理员仍可能需要打开 Casdoor 手工创建 Client，再复制信息到 Velora。

### 2.4 账号下发不是通用能力

当前配置只有：

```text
SpectraEnabled
SpectraURL
SpectraSecret
```

Worker 只消费 `velora.provisioning.spectra`。新增应用需要增加专属环境变量、修改 Worker 代码并重新部署 Velora，不符合产品化要求。

### 2.5 角色目录和用户授权写死 Spectra

身份服务只认可 Spectra 的固定角色集合；用户管理页面也只提供 Spectra 开关和 Spectra 角色选择。即使创建了第二个应用，其账号与角色也无法通过通用后台管理。

### 2.6 验证不是真正的全链路验证

现有 OIDC 验证只读取 Discovery 并比较 Issuer。它不能证明：

- Casdoor Client 与 Redirect URI 实际一致；
- 目标应用 Callback 已部署；
- Provisioning HMAC 签名和幂等逻辑正常；
- 测试账号可以登录；
- 无角色账号显示空态；
- 停用后 Session 被撤销；
- 退出能够清理应用与统一会话。

### 2.7 空访问策略默认放行

当前应用没有访问策略时按“所有人可访问”处理。生产产品应默认拒绝；确实面向全员时必须显式选择“全体成员”。

### 2.8 文档与产品脱节

现有文档分别出现“五分钟”和“十五分钟”接入说明，但实际仍依赖手工配置、应用专项开发和生产联调。文档应由接入向导生成应用专属配置，而不是让管理员在多份通用文档中拼装答案。

## 3. 产品边界

| 能力 | Velora | Casdoor | 目标应用 |
|---|---|---|---|
| 应用目录、负责人、分类、发布 | 负责 | 不负责 | 提供资料 |
| 账号生命周期和应用授权 | 负责 | 接收账号状态 | 接收签名事件 |
| 密码、MFA、SSO Session | 提供统一入口和策略编排 | 负责 | 不接触 |
| OIDC Client 和 Token | 保存公开绑定和状态 | 负责 | 校验 Token |
| OIDC Client Secret | 只通过一次性页面或加密短时 Handoff 转交，不进入业务数据库 | 生成 | 安全保存 |
| Provisioning Secret | 加密保存或保存 Secret 引用，用于签名 | 不接触 | 安全保存并验签 |
| 业务权限和数据权限 | 下发应用角色 | 不负责 | 最终负责 |
| 普通用户入口 | 唯一入口 | 永不暴露 | 默认跳 Velora SSO |

Casdoor 管理入口仅保留给具备专门权限的身份管理员用于应急排障。标准接入流程不得要求打开 Casdoor。

## 4. 目标用户流程

### 4.1 管理员看到的流程

1. **应用信息**：名称、不可变编码、负责人、图标、分类、生产域名。
2. **登录与账号同步**：默认选择“Velora 统一登录”，填写或确认 Callback、登出回跳和 Provisioning Endpoint。
3. **角色与访问范围**：维护应用角色目录，选择试运行用户/组；没有策略时保持默认拒绝。
4. **生成接入配置**：向导自动发起审批；批准后自动创建 Casdoor Client 和 Provisioning Secret，通过接入 CLI 或一次性页面安全交付。
5. **部署与自动检测**：应用团队部署配置，Velora 执行协议、下发和连通性检查。
6. **试运行与发布**：给测试账号下发权限，完成真实登录/退出，确认后发布。

普通管理员不需要理解或填写 Issuer、Scopes、Grant Type、PKCE、Provider Application Ref。它们显示在“高级设置”中且默认只读。

### 4.2 应用团队收到的交付物

接入包按应用生成，包含：

```text
README.md
velora-oidc.env.example
velora-provisioning.env.example
secrets/README.md
docker-compose.override.example.yml
kubernetes-secret.example.yaml
integration-checklist.md
rollback.md
```

提供两种交付模式：

1. **推荐模式：`velora-connect` CLI**。页面只展示五分钟有效、应用绑定、单次消费的 Enrollment Token；应用负责人在目标服务器执行 CLI，CLI 直接换取配置并以 `0600` 权限写入指定 Secret 文件。真实 Secret 不经过管理员复制粘贴。
2. **兼容模式：浏览器一次性显示**。适用于暂时无法使用 CLI 的环境；真实 Secret 只进入浏览器内存，不写入 localStorage、日志、审计、Git 或下载历史元数据。

两种模式关闭或消费后都只能执行轮换，不能再次读取旧值。普通配置模板与真实 Secret 分开下载，默认配置包不得包含明文 Secret。

### 4.3 状态模型

应用接入使用单一状态机：

```text
DRAFT
  -> APPROVAL_PENDING
  -> CREDENTIALS_ISSUED
  -> WAITING_FOR_DEPLOYMENT
  -> VERIFIED
  -> PILOT
  -> PUBLISHED
```

异常状态：

```text
ACTION_REQUIRED   配置或检测失败，可修复后重试
DEGRADED          已发布应用健康检查或下发持续失败
SUSPENDED         人工停用或安全停用
```

每个状态必须返回 `next_action`、阻塞检查项和最后一次有效配置版本。前端不根据零散字段自行推导状态。

### 4.4 角色和交接

一个接入任务只有一个负责人和一个当前处理人：

| 角色 | 产品内动作 |
|---|---|
| 应用负责人 | 提供域名、Callback、角色目录，部署接入包，处理应用侧失败 |
| 平台管理员 | 创建接入、配置访问范围、发起验证和发布 |
| 审批人 | 在 Velora 待办中批准凭据签发、高风险角色和生产发布 |
| 身份管理员 | 仅处理 Casdoor 自动化故障和应急恢复 |

向导自动创建并关联审批单，显示“等待谁处理”和处理入口；不允许要求管理员复制、粘贴 `approval_id`。

## 5. 数据模型改造

沿用现有 `portal_applications`、`portal_application_identity_bindings`、`portal_application_verifications` 和 `user_application_entitlements`，新增以下通用模型。

### 5.1 应用角色目录

`portal_application_roles`

| 字段 | 说明 |
|---|---|
| id / organization_id / application_id | 租户与应用隔离 |
| role_key | 应用内稳定角色编码，应用内唯一 |
| name / description | 管理员可理解的名称和说明 |
| risk_level | `NORMAL`、`PRIVILEGED`、`CRITICAL` |
| status | `ACTIVE`、`DISABLED` |
| config_version | 乐观锁版本 |
| created_at / updated_at | 审计时间 |

移除身份服务中的 Spectra 固定角色 map。所有授权必须读取已认证的应用角色目录；未知角色默认拒绝。

### 5.2 通用账号下发目标

`portal_application_provisioning_targets`

| 字段 | 说明 |
|---|---|
| application_id | 每应用一个默认目标，后续允许扩展多目标 |
| endpoint_url | 精确 HTTPS 地址 |
| signing_algorithm | V1 固定 `HMAC-SHA256` |
| secret_ref | 加密 Secret 标识或 Secret Provider 引用 |
| secret_fingerprint | 非敏感指纹，用于核对版本 |
| active_key_version | 当前签名密钥版本 |
| previous_key_version / previous_valid_until | 无中断轮换窗口 |
| delivery_status | `DISABLED`、`PENDING`、`HEALTHY`、`DEGRADED` |
| last_success_at / last_failure_at / last_error_code | 运维状态 |
| config_version | 乐观锁 |

Client Secret 不进入此表。Provisioning Secret 使用现有加密基础能力扩展为 `SecretStore` 接口：初期使用独立 KEK 的信封加密存储；接口预留 KMS/HSM/国密实现。数据库中不得出现明文。

### 5.3 接入检查与运行记录

`portal_application_onboarding_checks`

| 字段 | 说明 |
|---|---|
| application_id / config_version | 检查对应的应用配置版本 |
| check_type | 检查类型 |
| result | `PENDING`、`PASSED`、`FAILED`、`SKIPPED` |
| error_code | 稳定错误码 |
| evidence_json | 脱敏证据 |
| request_id / verified_by / occurred_at | 追踪信息 |

原 `portal_application_verifications` 数据迁入或兼容读取，避免同时维护两套结果来源。

### 5.4 一次性凭据交付

不建立保存明文 Secret 的业务表。凭据创建完成后按交付模式处理：

- CLI 模式默认只向浏览器返回 Enrollment Token；Client Secret 和 Provisioning Secret 使用独立 KEK 加密后放入单次消费、最长五分钟 TTL 的 Handoff Store，CLI 成功领取后立即删除；
- 浏览器兼容模式只在首次成功响应中返回 Secret，不建立 Handoff；
- Handoff Store 不能使用普通业务表，不得进入备份、日志、审计和通用缓存观测；过期未领取必须销毁并将接入状态标记为需要轮换。

一次性交付内容包括：

- OIDC Client Secret；
- Provisioning Secret；
- 两个 Secret 的版本和指纹；
- 可由前端内存生成的配置包内容。

幂等重放响应必须移除 Secret。用户丢失 Secret 时执行“轮换凭据”，产生新版本并保留短暂双密钥窗口。

CLI Enrollment Token 单独记录哈希、应用、组织、用途、过期时间、消费时间和创建人；服务端不保存 Token 明文。Token 不能用于登录、读取应用资料或执行其他管理 API。

### 5.5 接入操作与外部系统对账

`portal_application_onboarding_operations` 记录创建 Client、更新 Client、签发凭据、验证、暂停和发布等跨系统操作：

- `operation_type`、`desired_version`、`status`、`attempt_count`；
- `provider_request_id`、脱敏结果摘要和稳定错误码；
- `next_retry_at`、`created_at`、`completed_at`；
- 唯一幂等键，保证服务重启后可以继续。

数据库事务只提交本地期望状态和 Outbox 事件，外部 Casdoor 调用由可恢复操作执行器完成。定时对账比较 Velora 绑定和 Casdoor Client 的 Client ID、Redirect URI、状态与 Scopes；发现漂移时标记 `ACTION_REQUIRED`，不能静默覆盖人工修改。

## 6. 后端实施方案

### 6.1 保留并复用的能力

- 复用现有应用 CRUD、访问策略、身份绑定、发布、停用和审计框架。
- 复用现有 Casdoor Admin Client，不修改 Casdoor。
- 复用现有审批执行校验；凭据签发、Redirect URI 变更、停用和密钥轮换属于高风险动作。
- 复用 Reliable Message 表、幂等事件和当前 Provisioning HMAC 契约。
- 保留现有 API 一段兼容期，新向导通过聚合接口编排现有领域服务。

### 6.2 新增聚合接口

建议增加以下 HTTP/gRPC 能力，具体 Proto 命名遵循项目现有规范：

| 接口 | 用途 |
|---|---|
| `POST /api/v1/admin/application-onboardings` | 原子创建应用草稿和接入上下文 |
| `GET /api/v1/admin/application-onboardings/{id}` | 返回完整状态、检查项和下一步 |
| `PATCH /api/v1/admin/application-onboardings/{id}` | 保存基本信息、Callback、登出地址 |
| `PUT /api/v1/admin/application-onboardings/{id}/roles` | 替换角色目录 |
| `PUT /api/v1/admin/application-onboardings/{id}/policies` | 保存访问范围，默认拒绝 |
| `POST /api/v1/admin/application-onboardings/{id}/credentials` | 审批后创建 Client 和下发 Secret，一次性返回 |
| `POST /api/v1/admin/application-onboardings/{id}/credentials/rotate` | 安全轮换 |
| `PUT /api/v1/admin/application-onboardings/{id}/provisioning-target` | 配置下发地址 |
| `POST /api/v1/admin/application-onboardings/{id}/checks` | 运行全部可自动化检查 |
| `POST /api/v1/admin/application-onboardings/{id}/pilot` | 给指定测试用户投递测试授权 |
| `POST /api/v1/admin/application-onboardings/{id}/publish` | 所有强制检查通过后发布 |
| `POST /api/v1/admin/application-onboardings/{id}/suspend` | 下架入口并暂停新下发 |

所有写接口要求 CSRF、RBAC、组织隔离、幂等键、乐观锁和审计。敏感接口由向导自动创建审批并在批准后执行，服务端内部关联 `approval_id`；普通表单不展示、不要求输入审批编号。审批载荷只保存摘要和非敏感字段。

### 6.3 Casdoor 自动化

将现有可选自动化改造成标准路径，但启用前必须完成预检：

1. 自动化 Token 只存在 Server Secret 文件，绝不下发前端。
2. Token 权限限制为指定组织下的 Application 管理操作。
3. Provider Application Ref 和 Client ID 由服务端生成，应用编码作为稳定前缀。
4. Issuer 固定为配置中的 `https://auth.sevoniva.com`。
5. Scopes 固定为 `openid profile email`。
6. Grant Type 固定 Authorization Code，强制 PKCE S256、精确 Redirect URI。
7. 自动创建失败时本地保持 `ACTION_REQUIRED`，允许幂等重试，不伪成功。
8. Casdoor 不可用时不能发布新的 OIDC 应用，但不影响已发布应用目录读取。
9. 定时执行配置漂移检测；部分失败通过持久化 Operation/Outbox 恢复，禁止只依赖一次 HTTP 请求完成跨系统事务。

保留“绑定已有 Client”的高级模式，用于迁移或灾难恢复，不作为默认入口。

### 6.4 通用 Provisioning Dispatcher

替换专属 Spectra Dispatcher：

1. Reliable Message Topic 使用 `velora.provisioning.<application_id>` 或消息属性中的稳定 `application_id`，不在代码中枚举应用。
2. Worker 从数据库读取启用的 target，按应用解析 Endpoint 和 Secret 版本。
3. Target 配置采用短 TTL 缓存，并支持管理变更后主动失效；新增应用无需重启 Worker。
4. 每应用独立并发限制、熔断、退避和死信状态，避免单个应用拖垮其他应用。
5. 2xx 且返回合法确认视为成功；429/5xx 重试；明确契约 4xx 进入 `ACTION_REQUIRED` 并告警。
6. 保留事件幂等、聚合版本和顺序键语义。
7. 密钥轮换窗口内先用当前密钥，目标返回稳定签名版本错误时不得自动降级；旧密钥仅用于目标端验签兼容。

### 6.5 通用用户授权

改造用户创建和编辑接口，使 `entitlements[]` 真正支持多个应用：

```json
{
  "applicationCode": "spectra",
  "status": "ACTIVE",
  "roles": ["developer"]
}
```

服务端根据应用状态、角色目录和操作者授权上限校验：

- 应用必须存在且允许下发；
- 角色必须启用且属于该应用；
- `CRITICAL` 角色需要审批；
- 空角色是否允许由应用接入策略明确配置，默认允许登录但无业务权限；
- 停用授权必须下发空角色并撤销目标应用会话；
- 一个应用失败不得把其他应用授权事务错误地标记为成功。

### 6.6 默认拒绝迁移

上线前先为当前确实面向全员的已发布应用补充显式 `EVERYONE` 策略，再将无策略语义改为拒绝。迁移脚本必须输出变更清单并支持 dry-run，不能直接让现有应用突然消失。

### 6.7 Enrollment CLI

提供随发布制品构建的 `velora-connect`：

```text
velora-connect enroll --portal https://home.sevoniva.com --output /secure/<app>/
velora-connect doctor --config /etc/<app>/velora.env
```

要求：

- Token 不作为普通命令参数进入 Shell History；默认从交互输入、标准输入或临时只读文件读取。
- 校验门户 TLS、Token 受众、应用编码、目标目录和文件权限。
- 原子写文件，失败时不留下半份配置；默认权限 `0600`。
- `doctor` 只输出脱敏指纹、连通性和配置错误，不输出 Secret。
- CLI 只负责安全落地配置，不 SSH、不重启应用、不获得服务器管理权限。

## 7. 前端实施方案

### 7.1 页面结构

新增统一入口：

```text
/admin/applications
/admin/applications/new
/admin/applications/:id/onboarding
```

应用列表直接显示：接入状态、登录方式、账号同步状态、最近验证、负责人和下一步。现有“身份与单点登录”页逐步降级为高级运维视图，不能再成为普通接入主路径。

### 7.2 五步向导

1. **应用资料**：基础信息和生产地址。
2. **统一登录**：默认标准 SSO；根据应用基础 URL 推导 Callback，允许修改路径。
3. **账号与权限**：Provisioning Endpoint、角色目录、访问范围。
4. **接入配置**：审批、生成凭据、一次性复制/下载、显示部署检查清单。
5. **验证与发布**：逐项结果、可行动错误、测试用户、发布和回滚入口。

向导必须：

- 服务端保存草稿，换浏览器仍可继续；不把协议 Secret 写 localStorage。
- 每一步只有一个主按钮，明确显示下一步和阻塞原因。
- 技术字段进入高级抽屉，标准值默认只读。
- 离开含一次性 Secret 的页面前二次确认是否已保存。
- 错误显示稳定错误码、Request ID 和处理动作，不显示 Casdoor 内部地址或原始响应。
- 发布后仍能查看脱敏配置摘要、版本、检查历史和回滚说明。

### 7.3 用户管理页面

移除 `spectraEnabled`、`spectraRoles` 等专属前端状态。按已发布且启用 Provisioning 的应用动态展示授权卡片：

- 未开通；
- 已开通但无角色；
- 已开通并显示角色；
- 下发中；
- 下发失败，可重试；
- 已停用。

应用数量较多时采用按需搜索和抽屉编辑，不能在用户表格中无限增加列。

## 8. 标准接入 SDK 与样板

Spectra 保留为首个参考实现，但 Velora 仓库应提供通用契约和最小 SDK：

### 8.1 首期 Go SDK

- OIDC state、nonce、PKCE 和安全回跳；
- Discovery/JWKS、Issuer、Audience、Expiry、Nonce 校验；
- 服务端 Session 和联邦退出适配接口；
- Provisioning 请求大小限制、严格 JSON、恒定时间验签；
- 事件幂等、版本比较和结果响应类型；
- `integration.challenge` 处理；
- 标准健康与诊断信息结构；
- 示例测试和错误码。

SDK 不接管目标应用业务 RBAC、数据库表和 Session 存储实现，只提供安全边界和接口。

### 8.2 Reference App

保留一个最小 Reference App 用于持续集成，Spectra 用于生产级复杂样板。Reference App 必须在 CI 中完成自动接入、下发、登录协议拒绝测试和停用测试，避免后续修改只在生产发现问题。

## 9. 验证与发布门禁

### 9.1 自动检查

| 检查 | 发布门禁 |
|---|---|
| 应用编码、域名、Callback 静态校验 | 必须通过 |
| Casdoor Client 回读与配置比对 | 必须通过 |
| Discovery/JWKS/Issuer/TLS | 必须通过 |
| Provisioning Endpoint HTTPS 与签名 challenge | 必须通过 |
| 重复事件返回 `DUPLICATE` | 必须通过 |
| 旧版本事件返回 `STALE` | 必须通过 |
| 测试用户 ACTIVE 下发 | 必须通过 |
| 测试用户 DISABLED 与撤权确认 | 必须通过 |
| 访问策略非空或显式 EVERYONE | 必须通过 |
| Secret 交付确认 | 必须通过 |

### 9.2 人工交互检查

以下检查不能伪装成纯后端自动化，向导应提供“一键开始测试”并记录完成证据：

- 真实浏览器 SSO 登录；
- 登录后回到原路径；
- 无角色用户空态；
- 退出后返回 Velora 且不展示 Casdoor；
- break-glass `/normal-login` 不出现在普通导航。

自动检查和人工检查均绑定配置版本。配置发生变化后，受影响的旧检查自动失效。

## 10. 分阶段实施顺序

### Phase 0：契约和迁移护栏

目标：冻结边界，防止边改边产生新特例。

- 为现有 Spectra 行为补齐契约测试和数据基线。
- 增加数据库 migration dry-run、当前应用/策略/授权导出脚本。
- 定义状态机、稳定错误码、权限点和 Proto。
- 为当前全员应用补显式 EVERYONE 策略，再切换默认拒绝。
- 保留旧 API 和旧 Spectra Worker 开关作为回滚路径。

验收：现有 Velora 与 Spectra 行为不变；数据库升级和回滚演练通过。

### Phase 1：去除 Spectra 硬编码

目标：第二个应用无需修改 Velora 代码即可配置角色和下发。

- 新增通用角色目录和 Provisioning Target 数据模型、Repository、Service、API。
- 将身份服务角色校验改为数据库目录。
- 将 Worker 改为动态按应用路由。
- 用户管理页面动态展示多个应用授权。
- 迁移 Spectra 角色目录、Endpoint、Secret 引用和现有 Entitlement。
- 保证 Spectra Topic/配置兼容读取，完成后可回退旧 Dispatcher。

验收：创建一个非 Spectra 测试应用，只通过数据库/API 配置即可完成创建、更新、停用、重复和乱序事件投递。

### Phase 2：一站式向导与自动凭据

目标：管理员不打开 Casdoor 即可生成完整接入配置。

- 实现聚合 Onboarding API 和单一状态机。
- 重构应用管理为五步向导。
- 接入现有 Casdoor 自动化和审批能力。
- 向导自动创建、关联和等待审批，不再让用户填写审批编号。
- 自动生成 Client ID、Provider Ref、固定 Issuer/Scopes 和 Provisioning Secret。
- 实现一次性页面交付、Enrollment Token、接入 CLI 和安全轮换。
- 实现持久化 Operation/Outbox 和 Casdoor 配置漂移对账。
- 将访问策略放回接入上下文，默认拒绝。

验收：从空白草稿到“等待应用部署”只使用 Velora 页面；普通流程不出现 Casdoor 地址和非必要协议字段。

### Phase 3：全链路验证与发布闭环

目标：发布状态由证据决定，不再靠人工口头确认。

- 扩展验证引擎和版本化检查记录。
- 实现 Provisioning challenge、试运行账号、失败重试和状态展示。
- 实现真实登录/退出测试回执。
- 发布门禁检查全部通过后才允许发布。
- 实现应用级暂停下发、下架入口、恢复和回滚操作。
- 更新监控、指标和告警。

验收：新应用接入、正常授权、无角色、停用、恢复、退出和回滚全部有可查询证据。

### Phase 4：SDK 与开发者体验（产品化必需）

目标：降低目标应用开发量。

- 提供 Go SDK 和最小 Reference App。
- 提供稳定版 `velora-connect` CLI、校验和、版本兼容策略与发布制品。
- 从向导生成框架相关配置和代码指引。
- 后续按真实需求增加 Java、Node.js 等适配器，不预先堆积未使用 SDK。
- 合并现有三份接入文档，保留一份权威规范和一份排障手册。

验收：使用 Reference App 从领取 Enrollment Token 到通过自动检测，不需要复制 Secret、不需要修改 Velora 代码、不需要打开 Casdoor。Java、Node.js 等后续 SDK 不阻塞本项验收。

### 10.5 代码落点与建议提交切片

以下路径是按当前仓库结构确定的实施落点，开发时允许拆分文件，但不得改变责任边界：

| 纵向切片 | 主要落点 | 最小提交结果 |
|---|---|---|
| 数据与领域模型 | `server/internal/platform/database/migrations/postgres/`、`server/internal/domain/portal/` | 角色目录、Target、检查状态及迁移测试 |
| Repository 与服务 | `server/internal/adapters/repository/portal.go`、`identity_provisioning.go`、`server/internal/app/portal/`、`server/internal/app/identity/service.go` | 删除 Spectra 角色硬编码，支持通用应用授权 |
| 通用 Dispatcher | `server/internal/platform/provisioninghttp/`、`server/cmd/worker/main.go` | 动态目标、应用隔离、重试与兼容开关 |
| 接入 API | `server/api/proto/forge/v1/portal.proto`、`server/internal/adapters/kratosapi/portal.go` | 聚合状态、凭据、检查、试运行和发布门禁 |
| Casdoor 编排 | `server/internal/platform/casdooradmin/client.go` | 创建、回读、比对、停用和错误补偿 |
| 管理端向导 | `web/src/pages/admin/Applications.tsx`、`IdentityIntegrations.tsx`、`Policies.tsx`、`web/src/api/api.ts` | 单入口五步向导和高级运维视图 |
| 通用用户授权 | `web/src/pages/admin/Users.tsx` 及对应 Identity API | 动态应用、动态角色和下发状态 |
| 样板与文档 | Reference App、Spectra、`docs/` | 通用契约验证、权威接入文档和回滚手册 |

每个切片采用“测试先锁定旧行为 → 实现新能力 → 兼容迁移 → 自动化验收 → 提交并推送”的顺序。不能先一次性重写所有层再集中测试。

## 11. 测试要求

### 11.1 后端

- Domain：状态机、默认拒绝、角色目录、密钥版本、发布门禁。
- Repository：租户隔离、唯一约束、乐观锁、迁移兼容。
- API：权限、CSRF、幂等、审批、敏感字段脱敏、错误码。
- Casdoor Adapter：创建、回读、更新、停用、超时、重试、部分失败补偿。
- Dispatcher：多应用隔离、签名、429/5xx、4xx、超时、熔断、重放和顺序。
- Security：Secret 不进入日志、审计、缓存响应和幂等重放。

### 11.2 前端

- 五步向导正常、失败、恢复和刷新继续。
- 一次性 Secret 关闭确认与不可再次读取。
- 多应用角色动态渲染和大列表性能。
- 权限不足、审批待处理、自动化不可用和默认拒绝文案。
- 键盘操作、焦点顺序、表单错误关联、颜色非唯一状态表达。

### 11.3 集成和生产冒烟

- Reference App 全自动契约测试。
- Spectra 回归：登录、下发、角色、无权限、停用、恢复、退出。
- 第二个临时 Demo：证明无代码修改 Velora 即可新增目标。
- 生产真实浏览器测试绑定专用测试账号，测试后撤销授权。

## 12. 部署与回滚

### 12.1 部署顺序

1. 提交并推送 `main`，保存当前生产 commit、镜像摘要、数据库备份和配置摘要。
2. 部署 additive migration，新表和新字段先不接管旧流量。
3. 部署兼容读取的 Server/Web/Worker，旧 Spectra Dispatcher 继续工作。
4. 迁移 Spectra 角色目录、Target 和授权数据并执行对账。
5. 对 Spectra 开启通用 Dispatcher shadow 模式，只记录结果，不重复发送业务事件。
6. 对账通过后切换 Spectra 到通用 Dispatcher，保留旧开关。
7. 开启新向导，仅对管理员灰度。
8. 使用临时 Demo 完成全链路验收后再把新向导设为默认。

### 12.2 回滚原则

- 数据库 migration 必须 additive；事故窗口不删除旧列、旧表和旧数据。
- Phase 1 可切回旧 Spectra Dispatcher 和旧专属配置。
- Phase 2 可关闭新向导，继续使用现有应用 CRUD 和身份绑定 API。
- Casdoor 自动化失败时停用自动创建入口，不删除已存在 Client。
- 新应用发布失败时先下架入口、暂停该应用 Topic，再回滚目标应用配置。
- Secret 已签发后不得通过回滚恢复旧明文；必须轮换。

## 13. 安全要求

- 不修改或暴露 Casdoor 普通用户界面。
- 所有 Redirect URI 必须精确 HTTPS，禁止通配符、用户信息、Query 和 Fragment。
- Client Secret 不持久化在 Velora；Provisioning Secret 必须信封加密或存 Secret Provider。
- Secret 响应必须 `Cache-Control: no-store`，禁止写入前端日志、监控、埋点和 localStorage。
- 高风险角色、凭据签发/轮换、Callback 变更和应用停用必须经过审批与审计。
- 自动化 Token 最小权限，限制组织和允许操作。
- 所有查询和唯一约束包含 `organization_id`。
- 无策略、未知角色、未预配用户和验证状态未知全部失败关闭。
- 不向下游发送密码、MFA Secret、Cookie、OIDC Token 或 Client Secret。

## 14. 产品级体验指标

上线验收不只看功能完成，还必须记录以下指标：

| 指标 | 验收目标 |
|---|---|
| 普通链接应用管理员操作 | 2 分钟内完成 |
| 已采用 Go SDK 的 OIDC 应用管理员操作 | 10 分钟内完成接入配置并进入等待部署 |
| 标准流程技术字段手填数量 | 仅域名、Callback/Endpoint；其余自动生成或推导 |
| Secret 人工复制次数 | 推荐 CLI 路径为 0 |
| Casdoor 控制台访问 | 标准路径为 0 |
| 新应用引起的 Velora 代码/专属环境变量修改 | 0 |
| 自动检查失败可定位率 | 100% 提供稳定错误码、Request ID 和下一步 |
| 中断恢复 | 刷新、换浏览器或服务重启后可从服务端状态继续 |
| 外部系统部分失败 | 可自动重试、对账或明确进入 `ACTION_REQUIRED`，不得悬空 |

同时埋点接入漏斗：创建草稿、提交审批、领取凭据、首次部署检测、验证通过、试运行、发布和放弃原因。埋点不得包含域内用户资料、Secret、Token 或协议载荷。

## 15. 完成定义

只有同时满足以下条件，才可以宣称“应用接入已产品化”：

1. 新增应用不修改 Velora Go/TypeScript 代码。
2. 新增应用不增加应用专属环境变量或 Worker 分支。
3. 管理员标准流程不需要打开 Casdoor。
4. 用户管理支持任意已接入应用和动态角色目录。
5. 无访问策略默认拒绝，EVERYONE 必须显式选择。
6. OIDC、Provisioning、试运行和真实登录/退出均有版本化验收证据。
7. 一次性 Secret 不可从数据库、日志、审计或幂等重放中恢复。
8. Spectra 使用同一套通用机制且全部回归通过。
9. 第二个 Demo 应用在不改 Velora 代码的前提下完成生产式接入和撤回。
10. 接入文档、接口定义、配置说明、监控告警和回滚手册全部更新。
11. 标准流程不要求输入审批编号，审批在向导内创建、等待和完成。
12. 推荐 CLI 路径不需要管理员复制 Client Secret 或 Provisioning Secret。
13. Casdoor 调用部分失败可以从 Operation/Outbox 恢复并完成漂移对账。
14. 达到第 14 节的产品体验指标并保存验收证据。

## 16. 开发执行约束

- 全部在 `main` 开发，开始前确认工作区并提交当前有效修改作为回滚点。
- 按 Phase 0、1、2、3、4 顺序执行；每个 Phase 独立提交并推送远程，提交使用 Conventional Commits。
- 每完成一个可验证纵向切片立即测试、提交和推送，禁止积累大量未推送修改。
- 不删除或绕过现有生产安全校验来换取流程简化。
- 不为 Demo、Spectra 或未来应用新增名称硬编码。
- 不把 Mock 结果描述为生产验收；真实 Casdoor、真实数据库、真实浏览器和真实服务器验证必须明确区分。
- 发现方案与现有代码不一致时，以代码和测试证据为准，先更新本文再继续实现，禁止猜测接口或配置。
