# Velora 后端采用 Forge 脚手架实施方案

> 已被 [velora-backend-rapid-replacement-plan.md](velora-backend-rapid-replacement-plan.md) 取代。由于 Velora 当前代码和数据量较少，采用整体替换后端基座，不再按本文执行渐进迁移。

## 1. 结论

`go-antd-fullstack` 可以作为 Velora 新后端基座，且比继续扩展现有 Gin/GORM 后端更适合生产级、金融级建设。它已具备 Kratos v2、Proto/Buf/OpenAPI、`database/sql` + Goose、RBAC/数据范围、审批、可靠审计、OIDC 身份源、通用 S3、国密软件基线、独立迁移命令、供应链和部署门禁。

采用方式不是覆盖 `/Users/chuncheng/Downloads/code/velora/server`，而是以 `/Users/chuncheng/Downloads/code/go-antd-fullstack` 为新后端主干，将 Velora 的业务领域按纵向切片迁入。旧后端保留为接口和行为参考，前端暂不迁移。这样可以随时回滚，也避免把旧鉴权和基础设施技术债带入新基座。

当前判断为：工程可实施，迁移风险可控；脚手架属于“金融能力工程基线”，不是已经通过银行准入、等保或密评的成品。

## 2. 已验证事实与现存缺口

- 新脚手架约 24,506 行 Go/Proto、65 个 Go 测试文件；`make ai-governance` 和 `go test ./...` 已通过。
- Casdoor 可按标准 OIDC Discovery + Authorization Code Flow 接入；Velora 只做 OIDC Client，不修改 Casdoor，也不建设自有 OIDC Provider。
- 对象存储已经使用 AWS S3 SDK v2，支持 `generic-s3`、MinIO、腾讯云 COS 等 profile，并对高级能力执行目标证据门禁。
- 国密内置能力为 SM3 + SM4-GCM、版本化 Keyring；KMS/HSM/密码机是适配槽，真实投产仍需选定设备、实现适配器并完成密评证据。
- 原生产配置开启应用自动迁移的问题已修复，提交为 `1d779fe fix(config): forbid production auto migrations`。
- OIDC 浏览器回调必须补充标准 GET 路由；原实现只有 POST，直接配置 Casdoor 会导致回调失败。该项列为首个实施切片。
- 联邦登录当前要求预先建立 `(organization, provider, subject)` 映射，不会按邮箱自动绑定或自动创建用户，这是正确的金融级默认行为。

## 3. 固定架构边界

### 3.1 身份和权限

1. Casdoor 是唯一外部身份提供方，负责用户认证并签发 OIDC Token。
2. Velora 使用 `issuer + client_id + client_secret + redirect_uri` 接入，不调用 Casdoor 管理 API，不同步或修改 Casdoor 用户、密码、角色和配置。
3. 禁止迁移旧 Velora 的自建 OIDC Provider、ROPC 密码登录、Casdoor 管理端同步和修改密码逻辑。
4. 第一阶段只使用稳定的 `sub` 绑定本地账号；应用目录、收藏、访问策略等权限由 Velora 本地 RBAC/策略负责。
5. 如果以后要消费 Casdoor 的 `groups/roles/permissions`，必须先固定 Claim 契约并建立显式映射表；禁止直接信任任意 Claim 自动授予高权限。
6. 外部应用优先直接接 Casdoor；Velora 只负责门户目录、是否可见/可启动和审计，不代理下游应用的用户密码。

OIDC 是 Velora 与 Casdoor 之间采用的标准登录协议：浏览器跳转 Casdoor，用户完成认证后 Casdoor 携带一次性 code 回调 Velora，Velora 在服务端换取并验证 ID Token。

### 3.2 对象存储

业务只依赖统一 `Store/GovernanceStore`，不在领域代码判断厂商。开发可用 MinIO，生产可配置 COS 或其他 S3 兼容产品：

- MinIO：`provider=minio`、自定义 endpoint、通常 `path_style=true`。
- 腾讯云 COS：`provider=cos`、COS endpoint/region、按目标环境验证 path-style、签名和 TLS。
- 基础读写通过后才能启用；分片、校验和、SSE-KMS、版本、Object Lock、Retention、Legal Hold、STS 和预签名必须逐项取得 `Target-tested` 证据。
- 上传文件先进入隔离区，扫描通过后才能晋级为业务对象。

### 3.3 国密和密钥

- 开发/功能验证可使用内置 GM Provider；生产不能把代码内原始密钥当作 KMS/HSM。
- 领域层只依赖 Crypto Provider；外部 KMS/HSM/密码机通过 `KeySource`/Provider 适配器接入。
- 第一阶段保留标准加密默认值，同时把 GM 配置、密钥版本和轮换接口准备好。
- 确认具体密码设备后再实现 SM2、证书、国密 TLS、双人管钥、备份恢复和轮换演练，不提前绑定厂商 SDK。

## 4. 迁移范围

### 4.1 首批迁入

- 应用目录：应用、分类、标签、状态、排序、图标、启动地址和安全启动类型。
- 门户策略：组织范围、主体/角色/组与应用的访问策略，统一 `CanAccess` 判定。
- 用户行为：收藏、访问记录、最近访问；写入审计和必要的幂等控制。
- Casdoor OIDC 登录和外部身份绑定。
- 通用 S3 文件能力，供应用图标和后续附件使用。

### 4.2 暂缓

- 前端和旧前端接口兼容层。
- 邮件、待办、通知等非核心模块。
- 自动从 Casdoor Claim 授予角色。
- 真实 COS 高级能力、KMS/HSM 厂商适配、国密 TLS 和密评认证。
- 微服务拆分；首版保持模块化单体。

### 4.3 明确不迁入

- Velora 自建 OIDC Provider。
- Resource Owner Password Credentials 登录。
- Casdoor 管理 API 用户同步、修改密码和替 Casdoor 做账号生命周期管理。
- Gin/GORM 基础设施、应用启动自动迁移和开发 Compose 的生产化用法。

## 5. 实施阶段

### Wave 0：基座冻结与 Casdoor 可登录

交付：

1. 保持当前 `codex/velora-backend-foundation` 分支，不执行全仓库机械重命名。
2. 修复 OIDC GET 回调，同时保留 API POST 回调；要求 confidential client secret。
3. 增加 Casdoor 环境变量模板、TLS/Discovery/nonce/state/replay 测试和失败关闭说明。
4. 保持生产 `auto_migrate=false`；迁移只由 `velora-migrate` 一次性任务执行。
5. 输出本地 Casdoor 联调清单，不修改 Casdoor 服务本身。

验收：

- OIDC Discovery、跳转、GET 回调、code exchange、issuer/audience/signature/nonce 校验通过。
- state 重放、错误 issuer、错误 audience、缺少映射、Redis 不可用均失败关闭。
- 未配置 Casdoor 时其他后端能力仍可启动；生产不允许明文 HTTP 或缺失客户端密钥。

### Wave 1：应用目录最小闭环

新增 `portal` 领域，按 Domain → Application → Adapter → Transport 组织：

- `applications`、`application_categories`、`tags`、`application_tags`。
- `application_access_policies`，所有列表和启动操作统一调用 `CanAccess`。
- `favorites`、`application_visits`，唯一约束和幂等写入。
- PostgreSQL/MySQL Goose 前进迁移；优先 expand-only，不写破坏性 DDL。
- `portal.proto` 作为 HTTP/gRPC/OpenAPI 唯一契约源。

首批接口：

- `GET /api/v1/portal/applications`
- `GET /api/v1/portal/applications/{id}`
- `POST /api/v1/portal/applications/{id}/launch`
- `GET/POST/DELETE /api/v1/portal/favorites`
- 管理端应用、分类、标签和访问策略 CRUD

验收：跨组织隔离、越权、禁用应用、重复收藏、启动审计、并发写入和数据库回滚测试全部通过。

### Wave 2：存储与生产联调

- 应用图标接入统一 S3 Store，不在业务代码出现 MinIO/COS 分支。
- 建立 MinIO 开发契约测试和 COS 目标环境测试入口。
- 增加上传隔离、类型/大小/摘要校验和扫描适配槽。
- 完成独立迁移、备份、恢复、配置说明、Helm 渲染和回滚演练。

### Wave 3：兼容与切换

- 前端恢复开发后，根据实际调用补薄兼容层；核心领域不复刻旧 Gin handler。
- 新后端用独立数据库和端口运行，完成 API smoke、权限和审计验收后再切换前端 API 地址。
- 观察期内保留旧后端只读/可回切，不做双写；确认稳定后再下线旧服务。

## 6. 数据与回滚策略

当前业务数据较少，默认采用新库建表，不直接复用旧 schema。若存在需保留的数据，再编写一次性、可重复执行的导入工具，记录源数量、目标数量、失败项和摘要。

回滚原则：

1. 每个 Wave 独立 Conventional Commit，禁止把迁移、接口和大规模重构混在一个提交。
2. 新后端使用独立端口、数据库和配置；切换前旧后端不删除。
3. DDL 使用 expand/migrate/contract；Wave 0-2 不执行破坏性 contract。
4. 失败时停止新后端并把 API 地址切回旧后端；数据库保留现场用于分析。
5. 发布前创建数据库恢复点，记录 migration version；迁移失败阻止应用发布。

## 7. 测试和交付门禁

每个切片至少执行：

```bash
make ai-governance
make contract
make proto-check
make ci-go
```

阶段结束增加：`make security-tools`、迁移前进测试、备份恢复、Casdoor 浏览器联调、MinIO/COS 契约测试和 Helm 渲染。最终投产前运行 `make verify`，并补齐真实环境容量、长稳、故障、RTO/RPO、等保和密评证据。

## 8. Luna 执行提示词

```text
完整读取并严格执行仓库 AGENTS.md、.agents/skills/velora-banking-scaffold/SKILL.md、docs/ai-engineering-governance.md 和 docs/velora-backend-adoption-plan.md。

目标：以当前 go-antd-fullstack 为 Velora 新后端基座。不要修改 /Users/chuncheng/Downloads/code/velora，不要迁移前端，不要修改 Casdoor，不要实现 Velora 自建 OIDC Provider，不要实现 ROPC，不要调用 Casdoor 管理 API。

仅实施 docs/velora-backend-adoption-plan.md 的 Wave 0。先检查当前分支和工作区，保留已有修改；每个最小切片使用 Conventional Commit。完成 OIDC 标准 GET 回调并保留 POST、confidential client 配置校验、Casdoor 环境模板、OIDC 安全测试、生产自动迁移门禁验证、配置说明和回滚说明。

必须运行 make ai-governance、make contract、make proto-check、make ci-go。所有检查通过后汇报修改文件、提交、测试证据、未验证的目标环境事项和回滚方式，然后停止。未经确认不要进入 Wave 1。
```

## 9. 需要在 Wave 1 前确认的两个值

- 最终 Go module 地址，例如 `github.com/<organization>/velora`；确认前不运行 `make init`，避免无意义全仓库重命名。
- Casdoor 已存在客户端的 issuer、client ID、回调域名和实际 ID Token Claim 样例；只读取/配置，不要求改造 Casdoor。
