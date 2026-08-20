# Forge Banking Scaffold 使用文档

Forge 是面向企业、金融和信创交付的 Go 脚手架。默认采用模块化单体，只有通过领域拆分准入门禁后才拆成独立服务；前端采用 Shell + 受治理微前端。脚手架提供的是可复用的工程、审计、安全和交付边界，不等同于任何机构的等保、密评、监管或生产认证。

## 1. 首次启动

### 1.1 固定工具链

生产构建应使用仓库声明的版本和国内源策略：

- Go `1.26.6`
- Node.js 与 Corepack 管理的 pnpm
- Buf、Helm、Docker/Podman、kubectl
- PostgreSQL 或 MySQL；生产数据库版本必须进入目标环境证据矩阵
- Nacos 3、RocketMQ 5；Kafka 只作为已定义的消息提供方路径

不要在没有 ADR 和 Goal 变更的情况下替换固定框架、传输层或数据访问路径。

### 1.2 安装依赖

```bash
corepack enable
corepack pnpm install --frozen-lockfile --registry=https://registry.npmmirror.com
```

Go 依赖使用仓库 CI 约定的国内代理和校验策略。离线交付时，先准备内部 Go、npm、Buf、Helm 和容器镜像镜像库，再执行离线门禁；不能静默回退到未批准的外网源。

### 1.3 生成接口代码

Proto 是 HTTP、gRPC、OpenAPI 和前端 API Client 的唯一接口来源：

```bash
make proto-generate
```

不要直接修改 `api/gen` 或 `web/packages/api-client/src/generated` 下的生成文件。修改 `api/proto` 后重新生成，并让契约、Buf breaking 和前端生成检查通过。

### 1.4 本地验证

快速分层验证：

```bash
make ci-go
# 前端保持在父目录，使用仓库根目录的未修改前端门禁
cd .. && make test
```

合并前运行完整矩阵：

```bash
make ci-go && make ci-deploy && make security-tools
```

`make verify` 会覆盖接口契约、模块边界、Go 测试和竞态、前端 lint/typecheck/test/build、微前端边界、包体积、APISIX、可观测性、Helm、Gosec、漏洞、供应链、许可证、SBOM 和泄漏检查。

## 2. 配置和启动方式

配置按交付场景分层：

- `configs/minimal.yaml`：最小开发能力
- `configs/standard.yaml`：标准企业环境
- `configs/full.yaml`：平台能力和基础设施完整环境
- `configs/xinchuang.yaml`：信创部署约束和替换点

开发基础设施可以使用对应的 Compose 文件：

```bash
docker compose -f deploy/compose/standard.yaml up -d
```

`configs/minimal.yaml` 仅为本地开发保留 `database.auto_migrate: true`。所有生产 profile 均强制关闭它；生产发布必须先由同版本镜像中的 `velora-migrate` 一次性执行迁移，确认后才滚动 API/Worker。环境变量也不能在生产中重新打开该开关。

完整依赖环境：

```bash
docker compose -f deploy/compose/full.yaml up -d
```

生产交付使用 Helm，并且必须渲染信创 values 进行检查：

```bash
helm lint deploy/helm/velora -f deploy/helm/velora/values-xinchuang.yaml
helm template velora deploy/helm/velora \
  -f deploy/helm/velora/values-xinchuang.yaml
```

密码、令牌、数据库连接和对象存储密钥不能写入 Git。生产密钥进入批准的密钥管理系统或 HSM/KMS 适配槽；本地 `.env` 只用于开发。

## 3. 平台管理能力

平台管理后端是权限、数据范围、审批和审计的最终权威，前端按钮隐藏不是安全边界。通用能力包括：

- 用户、组织、部门、岗位和角色管理
- RBAC、数据范围、临时授权、访问复核和联邦身份源
- 密码策略、MFA、会话、登录安全和 API Token
- 审计查询、完整性校验、导出和归档
- 数据分类、脱敏、保留、删除审批和删除证据
- 配置变更版本、双人审批、发布、回滚和变更历史
- 作业状态、重试、死信、补偿、取消和审计

涉及权限、审批、密钥、审计、数据保留、对象存储和生产发布的失败，默认 fail-closed。

## 4. 通用 S3 使用方式

### 4.1 支持范围

存储层使用 AWS SDK for Go v2 的 S3 协议能力，但不把 MinIO 行为当成协议标准。适配器按能力协商工作，可接入：

- AWS S3
- MinIO
- Ceph RGW
- 阿里云 OSS S3 兼容接口
- 腾讯云 COS S3 兼容接口
- 华为云 OBS S3 兼容接口
- 其他通过目标契约测试的 S3 兼容存储

### 4.2 能力标签

- `Built-in`：代码内置的通用能力
- `Profile`：厂商或部署配置档案
- `Adapter slot`：需要接入机构组件的适配槽
- `Target-tested`：精确目标版本和配置已通过契约测试
- `Not certified`：没有有效目标证据，不能用于生产认证结论

基础读写和高级操作必须分别声明能力。校验和、SSE、KMS、版本控制、分片恢复、Object Lock、Retention、Legal Hold、STS 和受限预签名都不能因为“协议兼容”而自动视为支持。

### 4.3 合规对象写入

受监管对象使用 `GovernanceStore`，先通过目标能力门禁，再执行：

1. 服务端校验对象键、大小、类型和 SHA-256
2. 写入隔离区
3. 取得恶意软件扫描证据
4. 仅允许 `Clean` 对象晋级
5. 根据策略执行 SSE、版本、Retention、Legal Hold
6. 保留对象版本和审计证据

上传隔离使用 `QuarantineController`。生产环境必须提供数据库事务支持的 `QuarantineRecordStore`、真实隔离对象存储和机构批准的扫描器；测试替身不能作为投产证明。

STS、临时凭证、KMS/HSM 和供应商特有签名行为使用适配槽。未经精确目标环境测试，代码会拒绝高级操作，而不是降级成“看起来成功”。

详见 [storage-contract.md](storage-contract.md) 和 [providers.md](providers.md)。

## 5. 微前端接入

前端由 Shell 负责身份、权限、路由、菜单、主题、遥测和生产策略；业务微应用通过 manifest 注册。微应用必须满足：

- 来源、版本、完整性、权限和环境白名单可验证
- 默认采用隔离运行时，禁止未审批的任意远程脚本
- 通过 Host SDK 访问身份、事件和导航，不直接修改 Shell 内部状态
- 失败时可熔断、回退和审计，不能绕过后端权限
- 每个微应用有独立 lint、typecheck、test 和构建产物

示例远程微应用用于验证契约，不代表生产远程应用已经完成机构安全审批。

## 6. 配置变更、发布和回滚

### 6.1 配置变更

生产配置必须有版本、摘要、引用、敏感字段标识、审批人和审计事件。流程为：

`Draft -> PendingApproval -> Approved -> Published`

回滚必须引用目标版本，并重新经过权限、审批和审计门禁。敏感配置不能通过普通日志、前端列表或错误响应泄漏。

### 6.2 发布流程

`releasecontrol` 要求发布请求绑定：

- 制品 SHA-256 摘要
- SBOM 摘要
- 源码提交和来源证明
- 测试、漏洞和许可证结果
- 回滚方案
- 目标环境和变更窗口
- 至少两名不同审批人

状态流转为：

`Draft -> PendingApproval -> Approved -> Executing -> Succeeded`

回滚必须再次双人审批：

`Succeeded -> RollbackRequested -> RollbackApproved -> RollingBack -> RolledBack`

`ReleaseStore` 必须在同一个本地数据库事务中写入状态和审计事件；不能先改状态、再 best-effort 写日志。

详见 [release-governance.md](release-governance.md)。

## 7. AI 和协作规范

所有 AI 工具在修改仓库前必须读取：

1. 根目录 `AGENTS.md`
2. `.agents/skills/velora-banking-scaffold/SKILL.md`
3. `docs/ai-engineering-governance.md`
4. 当前 Goal 目标和 Git 忽略的本地可行性基线（如果存在）

每个完整切片必须：

- 遵守固定技术栈和模块边界
- 后端强制权限、数据范围、审批和审计
- 对安全、加密、审计、保留、存储和生产策略失败 fail-closed
- 使用国内或内部源，不静默外部回退
- 通过对应验证矩阵
- 使用 Conventional Commit 立即提交
- 保留用户改动，不使用 reset、checkout 覆盖、`--no-verify`、amend 或 rebase

AI 不得把本地 mock、协议兼容、静态检查通过或开发环境成功描述为银行生产认证。目标环境证据缺失时必须标记 `Not certified`，并列出需要补齐的真实证据。

## 8. 投产前必须补齐的真实证据

以下内容不能由脚手架静态代码自动代替：

- 目标数据库、Nacos、RocketMQ/Kafka 和 APISIX 版本验证
- 真实 S3 供应商、STS、KMS/HSM、Object Lock 和保留策略验证
- 真实恶意软件扫描器和隔离区清理验证
- 信创 CPU、OS、数据库、中间件和容器运行时兼容证据
- RPO/RTO、节点、网络、数据库、消息、对象存储、机房和备份恢复演练
- 机构审批的密评、等保、监管、数据跨境和审计留痕材料

这些证据完成并进入交付档案后，才能把对应目标从 `Not certified` 升级为 `Target-tested` 或机构批准的认证状态。
