# Velora 后端快速替换方案

> 本文替代渐进迁移思路。当前 Velora 后端代码和业务数据都少，目标是直接替换后端基座，不维护两套后端长期并行。

## 结论

直接把 Forge 后端能力迁入 `/Users/chuncheng/Downloads/code/velora/server`，替换现有 Gin/GORM 实现；`velora/web` 完全不动。旧后端只通过 Git 快照保留，不继续修补、不做双写、不逐接口迁移基础设施。

最终结构保持 Velora 现有仓库形态：

```text
velora/
├── web/                  # 原前端，暂不修改
├── server/               # 直接替换为 Forge 后端基座
│   ├── api/
│   ├── cmd/
│   ├── configs/
│   ├── internal/
│   ├── tools/
│   ├── go.mod
│   └── Makefile
├── deployments/          # 第二步切换到新后端镜像和迁移命令
└── docs/
```

Go module 固定为现有的 `github.com/sevoniva-labs/velora/server`，机械替换 Forge import path，不等待新的仓库地址。

## 替换原则

- 保留：`velora/web`、Git 历史、已有生产评估文档、产品业务定义。
- 整体替换：`velora/server` 的框架、配置、数据库访问、迁移、鉴权、审计和测试基座。
- 重新实现：应用目录、分类、标签、收藏、访问记录、门户访问策略。
- 不迁移：自建 OIDC Provider、ROPC、Casdoor 管理 API、Gin/GORM、邮件和待办。
- 不修改 Casdoor：只配置标准 OIDC Client。
- 不迁移历史数据：使用新数据库/schema；确有数据时再做一次性导入脚本。

## 快速执行顺序

### R0：整体换基座（半天到一天）

1. 将当前 Velora 工作区全部变更提交到快照分支，保证可以一键回滚。
2. 删除旧 `server` 实现并复制 Forge 的 `api/cmd/configs/internal/tools`、Go module、生成规则和后端脚本。
3. 将 module/import 从 `github.com/sevoniva-labs/forge` 改为 `github.com/sevoniva-labs/velora/server`。
4. 应用名、二进制名、环境变量前缀统一为 Velora；暂不做前端和全仓库品牌重构。
5. 保证 `go test ./...`、`go vet ./...`、Proto/合同门禁通过，服务能够启动并返回 health/readiness。

R0 完成即视为“后端已替换”，后续是在新基座开发业务，不再回到旧 Gin/GORM 后端加功能。

### R1：Casdoor 登录闭环（半天到一天）

- Casdoor 作为唯一外部 OIDC Issuer。
- 支持浏览器标准 GET callback，同时保留 JSON POST callback。
- 服务端使用 client secret 换 code，校验 issuer、audience、签名、nonce、一次性 state。
- 仅通过 Casdoor `sub` 绑定本地用户，不按邮箱自动合并，不调用 Casdoor 管理 API。
- 第一版权限放在 Velora 本地 RBAC/访问策略；Casdoor Claim 到角色的自动映射暂不做。

### R2：Portal 最小业务（两到三天）

一次完成一个 `portal` 模块：

- 应用、分类、标签及关联。
- 应用可见/可启动策略，统一 `CanAccess`。
- 收藏、最近访问和启动审计。
- Proto → HTTP/gRPC/OpenAPI → application service → repository → Goose migration 全链路。
- 管理接口和用户接口使用同一领域规则，不复制旧 handler。

前端仍不改；用 API 测试完成验收。

### R3：部署替换（一天）

- 生产强制 `auto_migrate=false`，发布前运行一次性 `velora-migrate`。
- 配置 Redis、PostgreSQL 和 S3；本地 MinIO，生产对象存储按 COS/其他 S3 profile 配置。
- 更新现有 Velora Docker/Compose/Helm 入口，只部署新后端二进制。
- 完成备份、恢复点、迁移、smoke、回滚演练。

## 对象存储与国密

- 对象存储直接沿用 Forge 的统一 S3 Adapter；业务代码不区分 MinIO/COS。
- MinIO/COS 的高级能力必须按目标环境测试；未验证的 SSE-KMS、Object Lock、STS 等保持关闭。
- 内置 GM 只作为 SM3/SM4-GCM 软件基线。
- KMS/HSM/密码机保留 Provider 接口，确认实际产品后再接，不阻塞本次后端替换。

## 验收门禁

R0 必须通过：

```bash
make ai-governance
make contract
make proto-check
go vet ./...
go test ./...
go test -race ./...
```

R1 增加 OIDC discovery/code/state/nonce/replay/错误 issuer/错误 audience/Redis 故障测试。R2 增加跨组织、越权、禁用应用、重复收藏、并发和事务回滚测试。R3 增加独立迁移、配置校验、备份恢复和容器 smoke。

## 回滚

- 替换前保存 Velora 当前分支和提交号。
- 新后端使用独立数据库或 schema，不覆盖旧库。
- R0-R2 失败：切回快照提交并启动旧后端。
- R3 切换失败：恢复旧镜像和旧数据库连接；新库保留用于排查，不反向写入旧库。
- 所有数据库变更优先 expand-only，确认新后端稳定前不执行破坏性 contract migration。

## 给 Luna 的快速替换提示词

```text
完整读取并严格执行 /Users/chuncheng/Downloads/code/go-antd-fullstack/AGENTS.md、.agents/skills/forge-banking-scaffold/SKILL.md、docs/ai-engineering-governance.md 和 docs/velora-backend-rapid-replacement-plan.md。

目标是迅速整体替换 Velora 后端，不做渐进迁移。目标仓库为 /Users/chuncheng/Downloads/code/velora；保持 web 目录不变，把 server 整体替换为 Forge 后端基座，Go module 使用 github.com/sevoniva-labs/velora/server。

先只实施 R0。开始前检查并保护 Velora 当前所有未提交修改，创建 codex/ 分支并提交可回滚快照。不要修改前端，不要修改 Casdoor，不要实现业务 Portal，不要进入 R1。完成后运行方案中全部 R0 门禁，提交最小 Conventional Commits，汇报替换文件、提交、测试、配置变化和回滚方式，然后停止等待确认。
```

## 总工期判断

在不迁移前端、没有重要历史数据、暂不接真实 KMS/HSM 的前提下：R0-R3 预计 4-6 个有效开发日。第一天即可完成后端基座替换，之后全部功能直接在新架构上开发。
