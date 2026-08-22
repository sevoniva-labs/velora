# Velora 与 Casdoor 产品边界及应用接入实施方案

> 状态：历史边界文档，账号与应用授权边界已由[《Velora 统一账号与应用接入标准》](./account-provisioning-and-application-onboarding.md)更新
>
> 适用范围：Velora 应用门户、管理后台、Casdoor 身份与单点登录集成
>
> 核心约束：不修改 Casdoor 源码；不恢复 Velora 自建 OIDC Provider；不让两套后台重复维护同一事实

## 1. 结论

Velora 与 Casdoor 采用“一个产品入口、两个专业控制面”的模式：

- **Velora 是所有用户和管理员的主要产品入口**，负责统一登录页面、应用目录、展示、分类、标签、负责人、启动地址、访问策略、发布、停用和业务审计。
- **Velora 是账号生命周期与应用授权控制面**，管理员在 Velora 创建、停用账号并分配下游应用角色。
- **Casdoor 是唯一认证事实来源**，负责密码、MFA、SSO Session、OIDC/SAML/CAS 客户端、Redirect URI、Client ID、Client Secret 和身份 Claims；账号状态由 Velora 通过受控服务端 API 同步。
- **普通用户不接触 Casdoor 管理后台，也不需要看到 Casdoor 产品名称**；面向用户统一使用“统一身份中心”品牌。
- **身份管理员可以从 Velora 进入 Casdoor 管理后台**，但入口必须单独授权，并通过内网、VPN、Cloudflare Access 或同等级访问控制保护。
- **OIDC/SAML/CAS 应用仍需在 Casdoor 注册客户端**；Velora 只保存非敏感引用、接入状态和验证证据，不复制 Casdoor 的完整配置，不保存下游应用 Client Secret。

最终产品形态不是“所有东西都放进 Velora”，也不是“让管理员在两套后台自行摸索”，而是：

1. 在 Velora 创建应用草稿；
2. Velora 根据接入类型生成配置清单；
3. 身份管理员通过受控入口前往 Casdoor 完成身份侧配置；
4. 回到 Velora 记录引用、执行验证；
5. 配置访问策略并发布。

## 2. 必须冻结的设计原则

### 2.1 单一事实来源

每一类数据只能有一个权威来源：

- 账号控制状态、平台角色和应用授权以 Velora 为准；密码、MFA、认证 Subject 和会话事实以 Casdoor 为准；
- 门户应用和访问策略以 Velora 为准；
- 下游应用自己的会话、业务权限和 Client Secret 由下游应用负责。

Velora 可以保存 Casdoor 的 `provider_application_ref`、公开 `client_id` 和验证状态，但不得允许管理员在两边修改同一份完整配置。

### 2.2 产品品牌隐藏不等于绕过标准协议

“Casdoor 不暴露”定义为：

- 普通用户看不到 Casdoor 管理后台；
- 普通用户界面不出现 Casdoor 产品名；
- 登录、自助安全页面使用企业自己的“统一身份中心”名称和域名；
- Casdoor 技术名称只在身份管理员页面作为次级技术信息出现。

生产下游应用仍使用标准 OIDC Authorization Code + PKCE。Velora 的统一登录页可以接收用户输入并通过后端调用 Casdoor 既有密码认证接口，密码只在 TLS 请求生命周期内存在；Velora 不保存密码，也不把该兼容入口扩展为自建身份系统。MFA、Passkey、风险认证和密码策略仍由 Casdoor 负责。

### 2.3 管理职责分离

应用运营管理员不应自动获得身份系统管理权；身份管理员不应自动拥有应用发布和审计删除能力；审计员应保持只读。

### 2.4 不做伪接入

界面上出现的接入类型必须真实可用。未完成协议闭环的 SAML、CAS、ForwardAuth 不允许以可保存、可发布的正式选项出现。

## 3. Velora 与 Casdoor 边界矩阵

| 能力 | 权威系统 | 管理入口 | Velora 是否保存 | 说明 |
|---|---|---|---|---|
| 用户、邮箱、手机号、头像 | Casdoor | 统一身份中心 | 只保存会话所需快照/外部 subject | 不允许双向编辑 |
| 组织、角色、用户组 | Casdoor | Casdoor 身份管理 | 保存映射和权限计算快照 | 撤权必须在批准时限内生效 |
| 密码、MFA、Passkey、恢复方式 | Casdoor | 统一身份中心 | 不保存 | Velora 不提供生产改密能力 |
| 登录认证 | Casdoor | 企业身份登录页 | 保存一次性交易和 Velora session | 标准 OIDC Code + PKCE |
| OIDC/SAML/CAS 客户端注册 | Casdoor | Casdoor 管理后台 | 保存公开引用和状态 | Secret 由下游应用安全保存 |
| 应用名称、图标、描述、负责人 | Velora | Velora 应用管理 | 完整保存 | 门户资产主数据 |
| 分类、标签、精选、排序 | Velora | Velora 门户管理 | 完整保存 | Casdoor 不参与 |
| 应用主页和启动地址 | Velora | Velora 应用管理 | 完整保存 | 强制 HTTPS 和允许列表策略 |
| 门户可见性和访问策略 | Velora | Velora 访问策略 | 完整保存 | USER/ROLE/GROUP/ORGANIZATION/EVERYONE |
| 下游业务权限 | 下游应用 | 下游应用后台 | 不保存 | 门户授权不能替代下游鉴权 |
| 应用发布、停用 | Velora | Velora 应用管理 | 完整保存 | 身份接入未验证不得发布 |
| Velora 会话和设备下线 | Velora | Velora 用户中心/管理后台 | 完整保存 | Casdoor 登出后同时清理本地会话 |
| Casdoor 管理入口 | Velora 提供入口，Casdoor 承载页面 | Velora 身份与单点登录 | 只保存配置 URL | 仅 `iam.console.open` 可见 |
| 门户操作审计 | Velora | Velora 审计后台 | 完整保存 | 应用变更、验证、发布均审计 |
| 身份系统操作审计 | Casdoor | Casdoor 审计能力 | 可保存关联事件 ID | 不伪装成 Velora 原生审计 |

## 4. 不同应用接入类型的处理方式

### 4.1 普通 URL 应用

- 不需要在 Casdoor 创建客户端；
- Velora 保存主页、启动地址、图标、分类和访问策略；
- 启动地址必须通过 HTTPS、域名允许列表和安全跳转检查；
- Velora 只控制门户可见性和启动，不代表下游系统已完成鉴权。

### 4.2 OIDC 应用

- 必须在 Casdoor 创建 OIDC Client；
- 下游应用生成并校验自己的 `state`、`nonce`、PKCE；
- Velora 的 `launch_url` 指向下游应用自己的登录发起地址或首页；
- Velora 不替下游应用拼接 Casdoor authorize URL；
- Velora 保存 Casdoor 应用引用和公开 Client ID，不保存 Client Secret；
- 完成身份侧配置和启动验证后才能发布。

### 4.3 SAML/CAS 应用

- 仅在 Casdoor 与目标应用真实完成协议互操作后开放；
- 未实施前在正式表单中隐藏；
- 若展示规划状态，必须禁用保存和发布，明确标记“暂未开放”。

### 4.4 ForwardAuth 应用

- Casdoor 负责用户登录身份；
- Velora 负责应用访问策略和 ForwardAuth 决策；
- 网关只信任 Velora 服务端返回的身份头；
- 必须完成可信网关、Header 剥离、Host/App 绑定和 403 E2E 后才能开放。

## 5. 管理角色和权限模型

### 5.1 目标角色

| 角色 | 主要权限 | 不应拥有的权限 |
|---|---|---|
| `application_admin` | 应用草稿、分类、标签、访问策略、提交发布 | Casdoor 控制台、用户/MFA 管理、审计审批 |
| `iam_admin` | 身份接入配置、打开 Casdoor、确认 Client 配置、执行身份验证 | 应用业务内容删除、审计篡改 |
| `auditor` | 查看审计、接入验证和发布记录 | 修改应用、修改身份配置 |
| `system_admin` | 紧急全局管理和权限委派 | 不得绕过审计和高风险审批 |
| `user` | 访问获授权应用 | 任何管理能力 |

### 5.2 目标权限

- `portal.application.read`
- `portal.application.manage`
- `portal.application.publish`
- `iam.integration.read`
- `iam.integration.manage`
- `iam.integration.verify`
- `iam.console.open`
- `audit.read`

前端菜单必须根据权限集合渲染，不能继续只依赖一个 `admin=true`。后端仍是最终授权边界，任何隐藏菜单都不能代替 API 权限校验。

### 5.3 高风险操作

以下操作至少需要重新认证或 step-up MFA；金融场景建议增加双人审批：

- 首次发布 OIDC/ForwardAuth 应用；
- 修改已发布应用的启动域名或 Redirect URI；
- 将访问策略改为 `EVERYONE`；
- 打开 Casdoor 管理控制台；
- 修改身份绑定或替换 Client ID；
- 停用核心业务应用。

## 6. 管理后台信息架构

Velora 管理后台保持主入口，建议结构如下：

```text
管理后台
├── 概览
├── 门户管理
│   ├── 应用管理
│   ├── 分类管理
│   ├── 标签管理
│   └── 访问策略
├── 身份与单点登录
│   ├── 接入概览
│   ├── 待配置应用
│   ├── 验证记录
│   └── 打开身份管理控制台
├── 平台
│   ├── 集成令牌
│   └── 系统状态
└── 审计
    └── 审计日志
```

### 6.1 Casdoor 管理入口

入口名称建议：

- 主标题：`身份管理控制台`
- 描述：`管理用户、组织、角色、MFA 和单点登录客户端`
- 技术信息：`身份提供方：Casdoor`
- 按钮：`打开身份管理控制台`

入口要求：

- 只对 `iam.console.open` 可见；
- 后端授权接口返回 URL，不从公开健康接口泄露管理地址；
- 生产地址必须为 HTTPS；
- Host 必须位于配置允许列表；
- 新标签页打开并带外部链接提示；
- 禁止 iframe 嵌入 Casdoor 管理后台；
- 禁止将 Casdoor 管理 Cookie 交给 Velora；
- 点击入口记录 Velora 审计，但不记录 Casdoor Cookie、Token 或 URL 查询参数。

建议新增配置：

```text
VELORA_CASDOOR_ADMIN_URL=https://iam-admin.example.com
VELORA_CASDOOR_ACCOUNT_URL=https://identity.example.com/account
VELORA_CASDOOR_ALLOWED_HOSTS=iam-admin.example.com,identity.example.com
```

## 7. 应用接入向导

### 7.1 生命周期

建议将业务启停状态和接入流程状态分开：

```text
DRAFT
→ IDENTITY_PENDING
→ VERIFICATION_PENDING
→ READY
→ PUBLISHED
→ DISABLED

任一步验证失败 → VERIFICATION_FAILED → 修复后重新验证
```

- `DRAFT`：仅保存基础信息，不出现在普通用户门户；
- `IDENTITY_PENDING`：需要身份管理员在 Casdoor 配置；
- `VERIFICATION_PENDING`：配置已填写，尚未完成验证；
- `READY`：技术验证通过，等待发布；
- `PUBLISHED`：正式展示并允许启动；
- `DISABLED`：管理员主动停用；
- `VERIFICATION_FAILED`：显示可执行的失败原因，不发布。

### 7.2 向导步骤

#### 步骤一：基础信息

- 应用编码；
- 应用名称；
- 描述和关键词；
- 图标；
- 分类和标签；
- 负责人和所属部门。

#### 步骤二：访问方式

- 普通链接；
- OIDC；
- ForwardAuth（通过能力开关开放）；
- SAML/CAS 未完成前不显示为正式选项。

#### 步骤三：身份接入

普通链接跳过本步骤。OIDC 展示：

- Casdoor Issuer；
- 建议的应用名称；
- 下游应用 Redirect URI 清单；
- 必要 Scopes；
- Client 类型要求；
- “打开身份管理控制台”按钮；
- Casdoor 应用引用；
- Client ID；
- 配置责任人和完成时间。

Client Secret 不允许输入 Velora。

#### 步骤四：门户访问策略

- 指定用户；
- 指定角色；
- 指定用户组；
- 指定组织；
- 全员；
- 无策略时的默认语义必须在界面明确显示。

#### 步骤五：验证与发布

检查项：

- 基础字段完整；
- 启动地址为批准的 HTTPS 域名；
- Casdoor Discovery 可达且 Issuer 精确匹配；
- Client ID 和 provider application reference 已记录；
- 下游应用能够发起自己的 OIDC 登录；
- 回调、state、nonce、PKCE 和登出 E2E 通过；
- 访问策略允许和拒绝场景通过；
- 审计事件完整；
- 高风险发布审批完成。

全部通过后才开放“发布应用”。

## 8. 数据模型方案

不建议继续把身份配置字段混在 `portal_applications`。采用独立绑定表，保持应用目录与身份协议解耦。

### 8.1 `portal_application_identity_bindings`

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | UUID | 主键 |
| `organization_id` | UUID | 租户隔离 |
| `application_id` | UUID | Velora 应用 ID，唯一绑定 |
| `provider_key` | varchar | 固定为 `casdoor`，保留扩展能力 |
| `protocol` | varchar | `OIDC`/`SAML`/`CAS`/`FORWARD_AUTH` |
| `provider_application_ref` | varchar | Casdoor 应用名称或稳定引用 |
| `public_client_id` | varchar | 公开 Client ID |
| `issuer` | varchar | 期望 Issuer |
| `redirect_uris_json` | text/json | 期望 Redirect URI 清单，不含 Secret |
| `configuration_status` | varchar | 配置状态 |
| `verification_status` | varchar | 验证状态 |
| `verified_at` | timestamptz | 最近验证时间 |
| `verified_by` | UUID | 验证管理员 |
| `verification_error` | varchar | 脱敏后的错误摘要 |
| `config_version` | bigint | 乐观锁/变更版本 |
| `created_at/updated_at` | timestamptz | 审计时间 |

唯一约束：

```text
(organization_id, application_id)
(organization_id, provider_key, provider_application_ref)
```

### 8.2 `portal_application_verifications`

保存每次验证证据：

- `application_id`；
- `binding_id`；
- `check_type`；
- `result`；
- `error_code`；
- `evidence_json`（脱敏、限长）；
- `verified_by`；
- `occurred_at`；
- `request_id`。

禁止保存：Client Secret、Access Token、Refresh Token、Authorization Code、Cookie、完整 ID Token。

### 8.3 应用生命周期字段

在 `portal_applications` 增加：

- `lifecycle_status`；
- `published_at`；
- `published_by`；
- `config_version`。

原有 `status=ENABLED/DISABLED` 暂时保留兼容，由迁移逻辑映射，不直接删除。

## 9. API 与 Proto 方案

建议新增或调整以下 API：

```text
GET  /api/v1/admin/identity/overview
GET  /api/v1/admin/identity/console-link

GET  /api/v1/admin/portal/applications/{id}/onboarding
PUT  /api/v1/admin/portal/applications/{id}/identity-binding
POST /api/v1/admin/portal/applications/{id}/verify
POST /api/v1/admin/portal/applications/{id}/submit-publish
POST /api/v1/admin/portal/applications/{id}/publish
POST /api/v1/admin/portal/applications/{id}/disable
```

关键规则：

- `console-link` 只对 `iam.console.open` 返回；
- 身份绑定写入要求 `iam.integration.manage`；
- 验证要求 `iam.integration.verify`；
- 发布要求 `portal.application.publish`；
- 已发布应用修改启动域名或身份绑定后自动回到 `VERIFICATION_PENDING`；
- 并发更新使用 `config_version`，拒绝覆盖他人刚提交的配置；
- 所有写操作使用幂等键和可靠审计；
- 错误响应不包含 Casdoor 内部地址、Token 或 Secret。

## 10. 当前实现必须修复的问题

### 10.1 Casdoor 字段是假保存

当前前端展示 `casdoorClientId` 和 `casdoorApplicationName`，但 `applicationBody` 没有提交这两个字段，后端 Proto 和数据库也没有对应字段。管理员会误以为配置成功。

处理：在身份绑定表和 API 完成前，立即隐藏这两个输入或显示“暂未接入”；完成后通过新 API 正式保存。

### 10.2 接入类型与运行能力不一致

当前前端允许选择 URL、OIDC、SAML、CAS、ForwardAuth，但服务端启动逻辑主要只验证并跳转 URL。

处理：

- URL 保持可用；
- OIDC 在完成绑定和 E2E 后开放；
- ForwardAuth 受能力开关控制；
- SAML/CAS 暂时隐藏。

### 10.3 普通用户界面暴露 Casdoor 名称

当前用户中心和无权限提示出现 Casdoor 产品名。

处理：

- 普通用户统一使用“统一身份中心”；
- 非管理员 403 提示改为“请联系身份管理员分配管理权限”；
- Casdoor 技术名称只在身份管理员页面展示。

### 10.4 前端只判断 `admin=true`

当前门户和管理后台入口使用笼统管理员布尔值，不能表达职责分离。

处理：后端 `/me` 返回权限集合，前端按权限显示菜单和按钮；API 继续独立做权限校验。

### 10.5 管理地址不应进入公开健康接口

账号自助地址与管理控制台地址必须分离。Casdoor 管理地址只能由受权管理接口返回，不能从公开 `/system/health` 暴露。

## 11. 安全与金融级控制要求

### 11.1 认证和凭据

- 下游应用生产使用标准 OIDC Code + PKCE；Velora 统一入口使用受控 Casdoor 密码认证 API；
- Velora 不保存 Casdoor 密码和下游 Client Secret；密码仅存在于 TLS 请求处理生命周期；
- 禁止日志记录 Authorization Code、Token、Cookie；
- 管理员会话采用短 TTL 和 step-up MFA；
- Casdoor 管理入口必须受网络边界和 MFA 保护。

### 11.2 管理后台

- 管理地址强制 HTTPS 和 Host 允许列表；
- 禁止 iframe、开放重定向和从请求参数传入目标 URL；
- Casdoor 控制台不能与普通门户共享公开访问策略；
- 不调用 Casdoor 管理员 API 修改用户、密码或角色；
- 第一阶段不自动创建 Casdoor 应用，避免引入高权限服务账号。

### 11.3 审批与审计

- 应用创建、身份绑定、验证、发布、停用均记录操作者、Request ID 和差异摘要；
- 身份配置人与发布审批人不得为同一人时，系统必须阻止；
- 审计失败时高风险操作失败关闭；
- 发布记录和验证证据纳入留存策略。

### 11.4 撤权

- Casdoor 用户停用、角色移除后，Velora 高权限会话必须在批准时限内失效；
- 高权限用户每次敏感操作重新检查权限版本或使用短会话；
- 下游应用仍需自行完成 Token/Session 撤销，Velora 不能替代。

## 12. 实施分阶段方案（已完成记录）

本节保留实施拆分和产品边界；本轮 P0—P4 已在新服务器完成，线上证据、提交记录和回滚步骤以[整体上线方案](production-clean-deployment-overall-plan.md)为准。后续只新增真实业务应用接入，不再重复建设 Casdoor 或 Velora OIDC Provider。

### Phase 0：边界冻结与界面止损

目标：消除假配置和产品误导。

- 固化本文档和架构引用；
- 隐藏未实现的协议选项；
- 隐藏当前不会落库的 Casdoor 字段；
- 普通用户文案统一改为“统一身份中心”；
- 增加权限集合返回模型，前端停止只依赖 `admin=true`。

验收：界面不存在“保存成功但后端未保存”的字段；普通用户界面不出现 Casdoor 产品名。

### Phase 1：数据模型与后端闭环

目标：保存身份绑定、验证状态和生命周期。

- 新增 additive migration；
- 新增 Proto/OpenAPI；
- 实现 repository、service、权限和可靠审计；
- 增加管理控制台链接接口；
- 增加配置校验和 Host 允许列表；
- 增加验证记录。

验收：Client ID 和 provider reference 可读回；Secret 无保存路径；越权访问全部 403。

### Phase 2：前端接入向导

目标：管理员可以按步骤完成应用接入。

- 应用草稿；
- 接入类型选择；
- 身份配置清单；
- Casdoor 受控入口；
- 访问策略；
- 验证结果；
- 发布和失败恢复。

验收：URL 应用无需 Casdoor；OIDC 应用未验证不能发布；不同角色只看到授权步骤。

### Phase 3：真实环境验证

目标：使用真实 Casdoor 和测试应用取得生产证据。

- Discovery；
- Authorization Code + PKCE；
- state/nonce；
- MFA；
- 错误 Redirect URI；
- 登出；
- 撤权；
- 允许/拒绝策略；
- 管理入口网络隔离；
- 审计链和告警。

验收：完整 E2E、错误场景和安全测试通过，不以 mock 代替真实证据。

### Phase 4：可选自动化

只有在手工流程稳定并取得批准后，才评估使用 Casdoor API 自动创建应用。

要求：

- 独立最小权限服务账号；
- 只允许应用客户端管理，不允许用户、密码和角色管理；
- Secret 一次性安全交付，不进入 Velora 日志或数据库；
- maker-checker 审批；
- 幂等、回滚、重试和审计完整。

Phase 4 不是当前上线前置项。

## 13. 迁移方案

### 13.1 旧应用处理

- 不删除现有应用；
- `launch_type=URL` 且地址合法的应用映射为 `PUBLISHED`；
- 标记为 OIDC 但没有正式绑定记录的应用映射为 `IDENTITY_PENDING`；
- SAML/CAS/ForwardAuth 存量应用映射为 `VERIFICATION_PENDING`，由管理员逐个确认；
- 当前未落库的 Casdoor Client ID 无法自动恢复，必须重新录入；
- 迁移前导出应用清单和校验和。

### 13.2 兼容策略

- 新表和字段采用 additive migration；
- 旧读取 API 在过渡期继续返回兼容字段；
- 新前端通过能力开关启用；
- 已发布 URL 应用不因上线向导而中断。

## 14. 回滚方案

配置开关：

```text
VELORA_APPLICATION_ONBOARDING_V2=false
VELORA_CASDOOR_ADMIN_ENTRY_ENABLED=false
```

回滚步骤：

1. 关闭 V2 向导和 Casdoor 管理入口；
2. 回退前后端镜像到上一兼容版本；
3. 保留 additive migration 和新表，不执行破坏性数据库回滚；
4. 旧应用继续按原有 URL 启动逻辑运行；
5. 导出回滚期间新增的草稿和身份绑定，待恢复后重放；
6. 验证登录、应用列表、启动、权限拒绝和审计。

禁止通过删除新表或覆盖数据库恢复来完成普通版本回滚。

## 15. 测试方案

### 15.1 后端

- 身份绑定创建、更新、读取；
- Client Secret 字段不存在；
- 生命周期状态机；
- 乐观锁冲突；
- 权限矩阵；
- 管理 URL HTTPS/Host 校验；
- 验证失败脱敏；
- 审计失败关闭；
- 迁移和兼容测试。

### 15.2 前端

- 按权限渲染菜单；
- URL/OIDC 向导分支；
- 未实现类型不可提交；
- 身份管理员入口可见性；
- 验证失败和恢复；
- 未验证禁止发布；
- 普通用户界面无 Casdoor 产品名；
- 键盘操作、焦点、错误提示和移动端布局。

### 15.3 E2E

- 应用管理员创建草稿；
- 身份管理员配置并验证；
- 发布审批；
- 普通用户按策略看到应用；
- 无权限用户看不到且直接 API 返回 403；
- OIDC 登录、MFA、登出和撤权；
- Casdoor 控制台对无权限管理员不可见且接口 403；
- 修改已发布 Client ID 后自动重新进入待验证状态。

## 16. 生产验收标准

满足以下全部条件才能标记完成：

- Velora 与 Casdoor 的数据权威边界无重叠；
- 普通用户不看到 Casdoor 管理后台或产品名；
- Casdoor 管理入口只对身份管理员开放；
- OIDC 应用必须经过身份配置、真实验证和发布审批；
- Velora 不保存下游 Client Secret；
- 未实现协议不可发布；
- 权限由细粒度 permission 控制，不只依赖管理员布尔值；
- 应用允许、拒绝、撤权和登出 E2E 通过；
- 所有高风险管理操作有可靠审计；
- 回滚演练通过且不需要破坏性数据库回退。

## 17. 预计代码影响范围

后端：

- `server/api/proto/forge/v1/portal.proto`
- `server/api/proto/forge/v1/system.proto` 或新增 identity administration proto
- `server/internal/platform/database/migrations/*`
- `server/internal/domain/portal/*`
- `server/internal/app/portal/*`
- `server/internal/adapters/repository/portal.go`
- `server/internal/adapters/kratosapi/portal.go`
- `server/internal/platform/authz/kratos.go`
- `server/internal/platform/config/config.go`

前端：

- `web/src/api/api.ts`
- `web/src/types.ts`
- `web/src/layout/menu.tsx`
- `web/src/layout/PortalLayout.tsx`
- `web/src/layout/AdminLayout.tsx`
- `web/src/pages/admin/Applications.tsx`
- 新增 `web/src/pages/admin/IdentityIntegrations.tsx`
- `web/src/pages/UserCenter.tsx`
- `web/src/AdminApp.tsx`
- `web/src/router.tsx`

配置与文档：

- `.env.example`
- `server/.env.example`
- `docker-compose.yml`
- `deployments/env/prod/.env.example`
- `deployments/env/prod/docker-compose.yml`
- `docs/architecture.md`
- `docs/ops-deploy.md`

## 18. 不在本方案内

- 修改 Casdoor 源码；
- 让 Velora 成为新的 OIDC Provider；
- 让 Velora 保存下游 Client Secret；
- 通过 iframe 或反向代理复制 Casdoor 管理后台；
- 第一阶段通过 Casdoor 管理 API 修改用户、密码、角色；
- 在协议未验证前宣称 SAML/CAS/ForwardAuth 已上线；
- 用门户隐藏按钮代替下游应用鉴权。

## 19. 后续业务应用接入顺序

1. 在 Velora 创建应用草稿并填写 HTTPS 启动地址；
2. 身份管理员在 Casdoor 创建 OIDC Client，配置精确 Redirect URI；
3. 回到 Velora 保存公开 Client ID/Issuer，执行 Discovery、JWKS、PKCE 和 ID Token 验证；
4. 配置 Velora 应用访问策略，完成无权限用户拒绝和回滚演练后发布；
5. 只有确有业务需求时，才为已验证能力增加自动化，禁止通过修改 Casdoor 源码扩大边界。

这套顺序可以保持 Casdoor 不改造、普通用户不感知其技术品牌，同时让身份管理员拥有必要入口，并让应用接入流程在 Velora 内形成可追踪、可验证、可审计的闭环。
