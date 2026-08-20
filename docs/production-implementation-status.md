# Velora 生产实施状态与 Go/No-Go 证据

更新时间：2026-08-21（最近代码证据：`0f2ee65`）

## 当前结论

当前分支可以进入开发/隔离预发联调，但仍不得宣称金融生产上线。代码侧生产硬化、前后端接口适配和自动化门禁已补齐；真实 Casdoor、目标对象存储、国密 KMS/HSM、HA/灾备、WORM/SIEM、外部安全与合规证明仍属于目标环境验收项。

## 已实施并已 push

- Casdoor Authorization Code + PKCE 前端入口、SPA `/auth/callback`、生产关闭密码登录；Velora 自建 OIDC Provider 有配置硬开关，生产为 true 时启动失败。生产 OIDC session TTL 强制不超过 1 小时；本地 session 撤销后，如 Casdoor discovery 提供 `end_session_endpoint`，后端返回标准 RP-initiated logout URL，前端再跳转清理上游会话。
- OIDC 事务缓存键改为 SHA-256(state)，一次性消费仍由 Redis compare-and-delete 保证；回调严格校验 HttpOnly 事务 cookie、state、nonce、PKCE 和 issuer/client。
- ForwardAuth：`GET /api/v1/auth/forward/{application_id}` 从可信路由参数加载应用，统一执行 `CanAccess`，返回后端签发的身份响应头；文档要求网关剥离入站 `X-Velora-*` 头。
- 生产配置 fail-fast：HTTPS public URL、精确 HTTPS origins、Secure Cookie、OIDC、ObjectStore KMS SSE、无 host port 的生产 Compose、Casdoor `initData=false`。
- CryptoProvider 已显式区分 `software` 与 KMS/HSM/PKCS#11 adapter slot；生产一律拒绝 software key storage，选择 `gm` 时同样拒绝软件信任根，未安装真实 adapter 会 fail-closed，不把软件 SM4 基线冒充商密硬件/认证能力。
- OIDC 模式下本地密码/MFA 管理后端拒绝，用户中心跳转 Casdoor 账号中心；未接入的待办、邮件、共享门户设置从生产 UI 隐藏或明确失败，门户设置页已改为只读并提示走版本化配置发布。
- 备份/恢复：`umask 077`、SHA-256 清单、可选 age 加密、上传失败非零、恢复前保险备份失败即中止、`pg_restore` 错误不再吞掉；新增 `backup-all-databases.sh` / `restore-all-databases.sh`，以同一时间戳编排 Velora 与 Casdoor 双库。
- 网关 `/healthz` 已代理 server readiness，依赖异常通过 HTTP 503 暴露；不再固定返回 200 冒充业务就绪。
- Helm 生产基线明确 `cryptoAdapter=hsm`，生产 Compose 强制注入 adapter；未安装批准的真实 adapter 会 fail-closed。
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

`make check-prod-config` 已通过；当前新增切片还执行了 Go focused tests、Web lint/test/build、Helm lint/template、shell `bash -n` 和 `git diff --check`。这些检查只做代码/静态门禁，不代表 Docker、Casdoor、数据库、对象存储或 KMS 真实环境已验收。

## 仍为 Go/No-Go 阻断项

以下没有目标环境证据前，结论必须是 **No-Go**：

1. Casdoor 真实 issuer 的 discovery、登录、MFA、logout、撤权、回调与账号中心 E2E；Velora external identity mapping 和角色撤销后的旧会话测试。
2. MinIO/腾讯云 COS 等最终对象存储的 multipart、checksum、私有 ACL、SSE-KMS、versioning、object-lock、presign、超时/重试契约测试。
3. 经过批准的国密 KMS/HSM/PKCS#11 adapter、密钥轮换、双人复核、旧密钥解密/rewrap、吊销和恢复演练。当前 `gm` 是软件算法基线，不是商密认证或真实 KMS/HSM。
4. 新增双库备份编排仍需在目标 PostgreSQL、对象存储和 Casdoor 上完成一致恢复点、PITR、异地不可变存储、连续恢复演练及批准的 RPO/RTO 证据。
5. 多实例、负载均衡、PostgreSQL/Redis HA、滚动发布、迁移 release job、故障注入和容量压测。
6. WORM/SIEM 链头锚定、职责分离、外部渗透/红队、等保/金融控制映射和签字证据。
7. 目标域名、TLS 证书链、DNS、网关 ForwardAuth 配置、Secret Manager 注入和生产值班/回滚演练。

OIDC 的上游 logout URL 只是标准协议跳转，是否真正清理 Casdoor 会话仍需目标 Casdoor 版本的真实 E2E 证明；本仓库不会伪造该证据。

## 回滚

- 代码：使用当前远端分支上一个已验证 commit/tag 构建旧镜像，生产 Compose 只回滚 `server/web` 镜像；保留新卷、日志和证据，不删除数据卷。
- 数据：按 `docs/ops-backup.md` 在隔离恢复环境演练后执行；禁止未经审批直接回滚生产数据库或 Casdoor 数据库。
- 配置：恢复上一份经审批的 Secret/配置版本，确认 `VELORA_OIDC_PROVIDER_ENABLED=false`、TLS、origins、crypto adapter 和网关路由后再滚动重启。
