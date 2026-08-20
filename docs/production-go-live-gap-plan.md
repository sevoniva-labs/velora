# Velora 生产级上线差距与实施方案

> 评估基线：`codex/velora-forge-backend-replacement`，提交 `002ca68`
>
> 评估时间：2026-08-21
>
> 结论性质：工程 Go/No-Go 评估，不替代等保、渗透测试、密码产品认证或监管验收。

实施状态、已验证门禁和回滚说明见 [`production-implementation-status.md`](production-implementation-status.md)。

## 1. 结论先行

当前项目**可以继续开发和做隔离预发，但不能正式生产上线（NO-GO）**。

最关键的事实是：

- 开发环境仍可显式使用 `VELORA_AUTH_MODE=password`，但生产配置 fail-closed 为 `oidc`；生产登录页只显示 Casdoor 入口，不把密码提交给 Velora。
- 前端已有 Casdoor Authorization Code + PKCE 跳转、SPA callback、错误恢复和 RP-initiated logout；后端校验 issuer、state、nonce、PKCE、一次性事务和本地 session 撤销。真实 Casdoor 互操作、身份关联、撤权和登出仍需目标环境证据。
- 根目录 `docker-compose.yml` 是开发编排，仍含默认凭据和多项 host port；不能作为生产编排。`deployments/env/prod/docker-compose.yml` 的静态边界较好，但必须纳入 CI 门禁并完成真实运行验收。
- 对象存储、国密 KMS/HSM、HA/灾备、不可篡改审计、监控告警、外部渗透和合规证据均未闭环。

### 当前成熟度（区间估计）

| 维度 | 当前状态 | 生产目标 | 差距 |
|---|---:|---:|---:|
| 业务接口与页面适配 | 85%–90% | 100% | P1：邮件/待办明确不在当前范围；共享设置已只读并需后续服务化 |
| 身份认证与授权 | 70%–80% | 100% | P0：真实 Casdoor E2E、撤权时限、外部身份生命周期证据 |
| 应用安全 | 70%–80% | 100% | P0/P1：真实网关、对象存储契约和外部测试证据 |
| 部署与可靠性 | 55%–65% | 100% | P0：HA 依赖、PITR、故障切换、SLO 运行证据 |
| 数据保护与金融控制 | 35%–45% | 100% | P0：批准 KMS/HSM、WORM/SIEM、职责分离、外部证明 |
| **综合判断** | **约 60%–70%（代码侧）** | **生产 Go** | **仍需目标环境联调、恢复/安全验收和签字证据，不能仅凭代码门禁 Go** |

以上比例是排期用的工程估计，不是合规评分。若平台将处理资金、账务或客户金融数据，还需要单独增加交易、对账、幂等、反欺诈/反洗钱等领域建设。

## 2. 生产 Go/No-Go 阻断项

下列任意一项未关闭，都不能把系统标为“正式生产可用”。

| 优先级 | 阻断项 | 必须实施 | 通过标准 |
|---|---|---|---|
| P0 | Casdoor 登录入口不完整 | 前端生产模式隐藏密码表单，只显示“使用 Casdoor 登录”；调用后端 begin 接口，处理 callback、state、nonce、PKCE、一次性消费和安全回跳 | 真实 Casdoor 登录、MFA、失败、重放、跨浏览器 callback、登出全部 E2E 通过；密码不经过 Velora |
| P0 | 身份关联与撤权不闭环 | 建立 `provider+subject` 关联生命周期；明确首次登录、禁用、角色/组织变化、离职、全部下线的处理；高权限会话短 TTL/权限版本校验 | Casdoor 撤权后旧 session/token 在验收时限内失效；未知 subject 默认拒绝；完整审计 |
| P0 | Velora 自建 OIDC Provider 仍在代码中 | 生产强制关闭 `/oidc/*`、`VELORA_OIDC`、新 client/key/token；不删除已发布 migration，待消费者盘点后另行清理 | 生产路由扫描无 Provider 入口；架构测试证明唯一 issuer 是 Casdoor |
| P0 | ForwardAuth/应用启动信任边界不足 | ForwardAuth 必须绑定可信 app-id/host 并统一执行 `CanAccess`；不得信任客户端伪造 header；目标 OIDC 应用由自身生成 state/nonce/PKCE | 直接访问、伪造 header、禁用应用、角色撤销、未知 app 五类绕过测试全部拒绝 |
| P0 | 默认凭据、明文入口和错误配置可进入生产 | 生产只发布受控网关 443（80 仅跳转）；DB/Redis/server/Casdoor/metrics/Grafana 仅内网；所有 secret 必填注入，禁止 `admin/admin`、`postgres/postgres` | `docker compose config` 和端口扫描均只暴露批准入口；缺少/使用 HTTP/localhost 的生产配置启动失败 |
| P0 | 密钥和国密能力没有真实信任根 | OIDC client secret、数据库、对象存储、备份、审计签名使用 Secret Manager；`gm` 只能接入已审计 KMS/HSM/PKCS#11 adapter，未配置真实 adapter 必须 fail-closed | 密钥轮换、旧密钥解密/rewrap、吊销和恢复演练通过；无明文私钥/密钥进入数据库或镜像 |
| P0 | 备份恢复不能证明可用 | Velora 与 Casdoor 一致恢复点、PostgreSQL PITR、加密/签名、异地/不可变存储；恢复脚本禁止吞错 | 新环境恢复演练达到业务批准的 RPO/RTO；登录、权限、审计、对象引用抽检通过 |
| P0 | 审计不可抵赖和高风险操作 fail-closed 未完成 | 业务变更与审计 outbox 同事务；签名/HMAC 使用 KMS/HSM；链头锚定 WORM/SIEM；审计写入失败时高风险变更不得成功 | 可验证、可检索、不可由单一 DB 管理员静默重算；故障注入后无“成功但无审计” |
| P0 | 供应链和外部安全证明缺失 | CI 增加 race、vet、govulncheck、SAST/SCA、secret、IaC/镜像扫描、SBOM、许可证检查；完成独立渗透测试 | Critical/High 清零或有批准的风险接受；发布包可追溯、可回滚 |

## 3. P1 产品与运行能力差距

这些不一定阻塞内部试点，但会阻塞“产品级完整上线”：

1. 邮件、待办当前不在 Wave 1 产品范围，生产 UI 不展示入口，API 适配层对写操作明确失败；若纳入产品范围，必须另立领域实现和 E2E。
2. 门户设置当前只读，生产修改必须走版本化配置发布；如需在线修改，必须增加鉴权配置服务、审计和审批。
3. MinIO 与腾讯云 COS 需要真实契约测试：multipart、checksum、私有 ACL、SSE-KMS、versioning、object-lock、presign、超时/重试；能力不支持必须显式降级或阻断。
4. PostgreSQL/Redis 当前仍需多实例、连接池、迁移锁、滚动发布、容量基线、故障切换和限流状态一致性验证。
5. 指标、日志、告警、SLO、合成监控和 runbook 需要真实通知链路；`/metrics` 只能内网访问，不能以固定 200 的网关健康页冒充业务就绪。
6. 登录请求体、限流容量、健康探针和启动 URL 的代码边界已加固；邮件/IMAP/Todo 仍未接入，不得以空实现宣称功能完成。
7. 需要建立数据分类分级、保留/删除/导出、更正、事件响应、季度权限复核和职责分离流程，并保存证据。

### 安全基线快照

只读基线扫描覆盖 82 个文件，发现 14 项需要处置的问题：1 项 Critical、6 项 High、6 项 Medium、1 项 Low。这里的“开发 Compose 默认数据库凭据”是根目录开发编排问题，不代表独立生产 Compose 已经暴露；但必须通过 CI 防止误用。

- High：ForwardAuth 和自建 OIDC Provider 的应用授权边界、管理员/应用权限快照最长七天、改密/全部下线后的 token 撤销、OIDC 私钥明文落库、开发/旧部署的明文端口、Grafana 默认管理员凭据。
- Medium：旧 OIDC 路径的浏览器绑定、IMAP 和健康检查 SSRF、登录请求体和 lockout 表无界增长、审计链可重算、审计失败仍提交业务变更、备份/归档权限与加密不足。
- Low：邮件正文的 CSS 远程 URL 仍可能绕过远程图片开关。

这些问题不能用“单元测试通过”抵消；必须在对应 Wave 中有修复、反例测试和部署/运行证据。

## 4. 推荐实施波次

原始 Wave 计划用于控制实施顺序。本轮已按用户授权完成代码侧 Wave 1–9 可落地整改；剩余项均是目标环境验收、供应商能力或合规证据，不应伪造为代码完成。

### Wave 1：生产基座与供应链门禁（代码侧已完成；目标环境验收待执行）

- 固化独立生产 Compose，禁止开发 Compose 合并污染。
- 强制 `VELORA_ENV=production`、HTTPS `PUBLIC_BASE_URL`、精确 `VELORA_ALLOWED_ORIGINS`、`TRUSTED_PROXIES`、Redis TLS、数据库 TLS。
- Secret Manager/只读 secret file；取消所有默认口令；Casdoor `initData=false`。
- 生产只允许网关入口；补端口、默认凭据、localhost/HTTP 配置静态检查。
- 完成依赖升级、govulncheck、SBOM、镜像/IaC/secret 扫描和可回滚镜像产物。

验收：`make test`、`go test -race ./...`、`go vet ./...`、`govulncheck ./...`、前端 lint/test/build、生产 Compose 最终配置检查、外部端口扫描、`git diff --check`。

### Wave 2：Casdoor 标准 OIDC 登录与前端适配（代码侧已完成；真实 Casdoor E2E 待执行）

- 登录页根据生产能力只显示 Casdoor SSO；开发密码模式显式隔离，不能带入生产。
- 增加 begin/callback 路由和前端 callback 页面；回跳只允许站内相对路径。
- 校验 issuer、state、nonce、PKCE、transaction cookie、一次性消费和 callback 重放。
- 用户资料、改密、MFA 跳转 Casdoor account；Velora 只清理本地 session 并执行标准登出。
- 真实 Casdoor discovery、MFA、claims、logout、错误页和移动端浏览器 E2E。

### Wave 3：身份授权与下游边界（代码侧基线已完成；生命周期/撤权验收待执行）

- 完成 identity link/首次登录/禁用/组织和角色映射。
- 统一 `CanAccess` 用于门户、Launch、ForwardAuth 和所有下游授权边界。
- 引入 session/token/revocation service；改密、停用、角色变更、全部下线必须可追踪撤销。
- 生产关闭 Velora OIDC Provider，清理管理后台相关入口，保留 migration 兼容。

### Wave 4：数据、对象存储、Crypto/KMS/HSM、备份审计（代码侧基线已完成；供应商契约与恢复演练待执行）

- S3-compatible adapter 用 MinIO 集成测试、COS 预发契约测试。
- 接入经批准的 KMS/HSM/国密 provider；完成 envelope 版本化、双 key 解密、rewrap、密钥轮换。
- Velora/Casdoor 统一备份、PITR、加密、签名、异地不可变存储和恢复演练。
- 审计 outbox、HMAC/签名、WORM/SIEM 锚定、归档 manifest 和故障 fail-closed。

### Wave 5：HA、可观测性、性能和外部证明（部署基线已具备；运行/外部证明待执行）

- 多实例、负载均衡、滚动发布、迁移 release job、Redis/PostgreSQL HA。
- SLI/SLO、告警、证书/磁盘/DB/Redis/备份监控、runbook、合成监控。
- 2 倍峰值容量测试、故障注入、恢复演练、核心旅程 Playwright E2E、WCAG 2.2 AA。
- 独立渗透测试、红队、等保/金融控制映射，关闭或正式接受所有 High 风险。

## 5. 生产配置基线

生产部署至少必须明确并经过审批：

```text
VELORA_ENV=production
VELORA_AUTH_MODE=oidc
VELORA_OIDC_ISSUER=https://<approved-casdoor-issuer>
VELORA_OIDC_REDIRECT_URL=https://<public-host>/auth/callback
VELORA_ALLOWED_ORIGINS=https://<public-host>
VELORA_SECURE_COOKIES=true
VELORA_TRUSTED_PROXIES=<approved-proxy-cidr>
VELORA_DATABASE_DSN=<TLS + least-privilege app account>
VELORA_REDIS_TLS=true
VELORA_STORAGE_SSE_MODE=kms
VELORA_CRYPTO_PROVIDER=<approved-provider>
VELORA_OIDC_PROVIDER_ENABLED=false
```

`VELORA_BOOTSTRAP_PASSWORD_FILE` 只能作为受控 break-glass 流程存在：不用于日常登录、不出现在镜像/日志、不允许无限期有效，并必须有双人审批、轮换和使用审计。

## 6. 验收、发布与回滚

### 发布前 Go 门禁

- P0 = 0；未接受的 High = 0。
- Casdoor 真实 OIDC/MFA/logout/撤权 E2E 全通过。
- 安全扫描、SBOM、镜像签名和依赖许可证检查通过。
- 生产端口、TLS、密钥、备份、恢复、WORM 审计和告警均有证据。
- RPO/RTO、SLO、容量上限、值班人、事件升级路径和 runbook 已批准。
- 产品范围内每个模块都有成功、空、错误、慢、无权限和移动端验收；未完成模块必须隐藏并出现在发布说明。

### 回滚原则

1. 发布前创建镜像 digest、配置版本、数据库/IdP 一致恢复点和审计 manifest。
2. 应用异常优先切回上一镜像和上一配置，保留数据库向前兼容；禁止直接回滚已执行 migration。
3. OIDC 异常时关闭新流量、保留旧版本或维护页；不得把 ROPC 作为生产“临时回退”。
4. 数据异常时在隔离环境恢复验证，再按批准的 RPO/RTO 切换；切换后吊销受影响 session/token 并补审计。
5. 回滚完成后执行健康、登录、权限、审计和关键业务 smoke，并记录事故/变更单。

## 7. 最终判断

当前状态是：**代码侧后端替换、前端接口适配和生产硬化已完成到可进隔离预发；正式金融生产仍 No-Go**。

下一步不是继续堆“假完成”代码，而是使用目标 Casdoor、MinIO/COS、批准 KMS/HSM、生产网关和备份对象存储完成真实 E2E、恢复演练、故障注入、渗透测试与合规签字。任何一项 P0 证据未通过，都保持 No-Go。
