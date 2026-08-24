# Velora

Velora 是企业统一应用门户和身份治理平台：员工从一个入口登录并访问获授权应用；管理员在 Velora 完成组织、人员、应用、角色、使用范围、账号下发、审批和审计管理。

Casdoor 只作为隐藏的认证协议引擎。项目不修改 Casdoor 源码，不把 Casdoor 后台作为日常产品入口，也不建设 Velora 自有 OIDC Provider。

## 当前能力

- Velora 品牌登录页、Turnstile、OIDC Authorization Code + PKCE、MFA 和服务端 Session。
- 组织、部门、岗位、用户组、平台角色、权限、会话和账号生命周期。
- 应用目录、分类、角色、部门/组/角色/人员使用范围、排除规则和默认拒绝。
- 应用多环境 Callback、OIDC Client 自动审批与创建、一次性安全接入包。
- HMAC 账号下发、可靠重试、幂等和单调版本。
- 每应用独立组织/部门/授权用户目录接口，支持全量和增量同步。
- 审批、临时权限、权限复核、审计完整性、配置变更和生产门禁。
- PostgreSQL、Redis、RocketMQ、S3 兼容对象存储、Docker Compose 和 Helm 基线。

## 项目结构

```text
server/        Go/Kratos 后端、Worker、迁移、SDK 和部署基座
web/           React、ProComponents 管理后台与员工门户
demo/          OIDC Reference App
deployments/   Velora 生产/预发 Compose
docs/          当前有效的产品、接入、运维和安全文档
```

## 本地开发

中国大陆环境先执行：

```bash
make bootstrap
make init
make dev-server
make dev-web
```

默认端口：Web `5173`，API `8080`。生产配置不得使用本地默认 Secret。

## 验证

```bash
GOPROXY=https://goproxy.cn \
NPM_REGISTRY=https://registry.npmmirror.com \
make verify

make check-prod-config
```

`make verify` 包含 Go 测试与竞态检查、前端测试与构建、Playwright E2E、Proto/OpenAPI 契约、部署检查、安全扫描和供应链证据。

## 文档

- [文档入口](./docs/README.md)
- [应用接入手册](./docs/application-integration-guide.md)
- [架构与产品边界](./docs/architecture.md)
- [生产运维](./docs/operations.md)
- [安全与合规边界](./docs/security-and-compliance.md)
- [生产就绪状态](./docs/production-readiness.md)

接口契约：[`server/api/gen/openapi/openapi.yaml`](./server/api/gen/openapi/openapi.yaml)。Go SDK：[`server/sdk/velora`](./server/sdk/velora)。

## 生产声明

当前实现是受控单机企业生产基线，不等于金融机构最终认证。多节点高可用、异地灾备、真实国密 KMS/HSM、WORM/SIEM、外部渗透、等保和密评必须在目标环境完成独立验收。
