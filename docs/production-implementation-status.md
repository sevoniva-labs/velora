# Velora 生产实施状态与 Go/No-Go 证据

更新时间：2026-08-21

## 当前结论

当前分支可以进入开发/预发联调，不得直接宣称金融生产上线。代码侧生产硬化与自动化门禁已补齐；真实 Casdoor、目标对象存储、国密 KMS/HSM、HA/灾备、WORM/SIEM、外部安全与合规证明仍属于目标环境验收项。

## 已实施并已 push

- Casdoor Authorization Code + PKCE 前端入口、SPA `/auth/callback`、生产关闭密码登录；Velora 自建 OIDC Provider 有配置硬开关，生产为 true 时启动失败。
- ForwardAuth：`GET /api/v1/auth/forward/{application_id}` 从可信路由参数加载应用，统一执行 `CanAccess`，返回后端签发的身份响应头；文档要求网关剥离入站 `X-Velora-*` 头。
- 生产配置 fail-fast：HTTPS public URL、精确 HTTPS origins、Secure Cookie、OIDC、ObjectStore KMS SSE、无 host port 的生产 Compose、Casdoor `initData=false`。
- OIDC 模式下本地密码/MFA 管理后端拒绝，用户中心跳转 Casdoor 账号中心；未接入的待办、邮件、共享门户设置从生产 UI 隐藏或明确失败。
- 备份/恢复：`umask 077`、SHA-256 清单、可选 age 加密、上传失败非零、恢复前保险备份失败即中止、`pg_restore` 错误不再吞掉。
- 审计导出按真实 `audit_logs` schema 生成 CSV/清单/元数据，默认不删除；删除必须走 WORM receipt 与 `audit_chain_anchors` 的应用归档流程。
- 本地限流状态有 10,000 key 上限并清理过期条目；CORS 不信任伪造 `X-Forwarded-Proto`。
- 生产告警改用当前真实 `forge_http_requests_total` 指标，路由标签低基数归一，移除未实现邮件告警。
- 根 `make verify` 已接入后端完整门禁；CI 增加生产 Compose 静态检查。

## 已执行的代码门禁

`cd server && make verify` 已通过：

- API error/proto/OpenAPI/security/module boundary checks
- `go mod verify`、`go vet`、`go test`、`go test -race`
- 前端 `pnpm install --frozen-lockfile`、lint、32 tests、build、bundle budget
- Helm lint/template、APISIX/observability policy
- gosec（0 issues）、govulncheck（0 可达漏洞）、staticcheck、golangci-lint（0 issues）
- supply-chain evidence generation

`make check-prod-config` 已通过；该检查只做静态 Compose 解析，不代表 Docker、Casdoor、数据库、对象存储或 KMS 真实环境已验收。

## 仍为 Go/No-Go 阻断项

以下没有目标环境证据前，结论必须是 **No-Go**：

1. Casdoor 真实 issuer 的 discovery、登录、MFA、logout、撤权、回调与账号中心 E2E；Velora external identity mapping 和角色撤销后的旧会话测试。
2. MinIO/腾讯云 COS 等最终对象存储的 multipart、checksum、私有 ACL、SSE-KMS、versioning、object-lock、presign、超时/重试契约测试。
3. 经过批准的国密 KMS/HSM/PKCS#11 adapter、密钥轮换、双人复核、旧密钥解密/rewrap、吊销和恢复演练。当前 `gm` 是软件算法基线，不是商密认证或真实 KMS/HSM。
4. Velora 与 Casdoor 的一致备份/PITR、异地不可变存储、连续恢复演练及批准的 RPO/RTO；应用数据库备份脚本不替代 Casdoor 备份策略。
5. 多实例、负载均衡、PostgreSQL/Redis HA、滚动发布、迁移 release job、故障注入和容量压测。
6. WORM/SIEM 链头锚定、职责分离、外部渗透/红队、等保/金融控制映射和签字证据。
7. 目标域名、TLS 证书链、DNS、网关 ForwardAuth 配置、Secret Manager 注入和生产值班/回滚演练。

## 回滚

- 代码：使用当前远端分支上一个已验证 commit/tag 构建旧镜像，生产 Compose 只回滚 `server/web` 镜像；保留新卷、日志和证据，不删除数据卷。
- 数据：按 `docs/ops-backup.md` 在隔离恢复环境演练后执行；禁止未经审批直接回滚生产数据库或 Casdoor 数据库。
- 配置：恢复上一份经审批的 Secret/配置版本，确认 `VELORA_OIDC_PROVIDER_ENABLED=false`、TLS、origins 和网关路由后再滚动重启。

