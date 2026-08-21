# Security Baseline

## 身份、权限与三员分立

- Browser：服务端 Cookie Session + HttpOnly + CSRF，Session token 仅存哈希。
- Machine：Bearer API Token，原文只在创建时返回一次，数据库仅存 SHA-256 hash；支持 scope 和撤销。
- RBAC 使用 Permission，不在 handler 中散落角色字符串。
- 默认角色：`system_admin`、`security_admin`、`auditor`、`user`，用于“三员分立”起点；角色按组织隔离，Permission 标识全局定义；具体岗位按组织制度继续细分。
- 提供 TOTP 注册、确认、加密存储、一次性恢复码和登录强制校验闭环；组织级强制策略与高风险操作 step-up 仍须按部署制度启用和验证。

## 口令与会话

- Argon2id；最小长度、大小写/数字/符号、历史口令、有效期均可配置。
- 首个管理员无源码默认密码；首次登录强制改密。
- 连续失败锁定 + 登录限流。
- 改密后撤销其他会话。
- Cookie 的 Secure/SameSite 可配；生产 HTTPS 下应启用 Secure。

## HTTP/API

- CSP、HSTS、X-Frame-Options、nosniff、Referrer-Policy、Permissions-Policy。
- CORS 精确 Allowlist；不使用带凭据的 `*`。
- 全局 body size limit；上传业务需继续校验文件头/MIME/压缩炸弹/恶意文件。
- 统一错误响应不回传内部 stack/SQL/底层连接信息。
- 6 位稳定返回码与 HTTP status 分离，响应携带 request_id / trace_id。
- pprof 默认关闭，`disable_debug_endpoints=true` 时即使配置误开启也不会注册。

## 数据与密码

- Standard：SHA-256 + AES-GCM。
- GM baseline：SM3 + SM4-GCM。
- 应用密文携带 key version 前缀；keyring 支持新版本加密、旧版本解密的轮换窗口，未知版本 fail-closed。
- 主密钥要求至少 32 字节或等价 base64；短口令不会被静默 hash 成“看似可用”的主密钥。
- 生产金融密码方案通常还需要 SM2、证书体系、KMS/HSM/密码机、双人管钥、密钥生命周期和密评；KMS/HSM 仍是 `Adapter slot`，扩展位见 `integrations/secrets/hsm`。
- `security/masking` 提供手机号、证件、银行卡、邮箱、姓名脱敏 primitive；真正脱敏策略由数据分类分级决定。
- `security/datapolicy` 提供字段目录策略边界；未登记字段不能被策略层导出，个人信息和受限字段强制脱敏、审批和水印。

## 审计与日志

- 登录、退出、改密、用户/Token 管理等安全事件写独立 audit 表。
- 审计按 organization 隔离；系统级未知组织登录失败不会被任意租户列表查询到。
- slog JSON/Text 统一输出，敏感 key 兜底 redaction；HTTP access log 不记录 query string。
- `mlps3/financial` profile 阻止网络日志留存天数配置低于 183 天。
- 长期留存、防篡改、WORM/SIEM 由生产日志平台实现，不建议用单一业务数据库承担全部网络日志归档。
- 业务审计日志包含事务内完整性链和校验接口；清理前必须由 WORM 归档适配器写入 Object Lock/Retention/Checksum/VersionId 证据，并在本地事务持久化 receipt 和 chain anchor。未配置或未验证适配器时 fail-closed。
- 审计可靠汇聚使用本地 `reliable_messages` outbox 与审计记录同事务写入；只有 RocketMQ allowlist 明确包含 `audit-events` 时才启用转发，发布失败进入重试/死信，不静默降级。

## 多实例与可靠性安全

- Redis Provider 时登录限流/分布式锁跨 Pod 生效；Memory Provider 只用于单实例语义。
- 幂等记录避免关键写操作重复执行；业务需在具体 endpoint 显式应用。
- 本地可靠消息表与业务状态在同一数据库事务提交，避免数据库与远端消息双写不一致。
- 依赖提供 timeout/retry/circuit/bulkhead primitive；业务必须按“只对幂等操作安全重试”的原则使用。

## Secret

DSN、Crypto Key、Redis/RocketMQ/Kafka Streaming/Search/S3/Nacos 凭据仅从环境变量或 `*_FILE` 读取。Kubernetes 推荐 External Secrets/CSI/组织 Secret 平台创建 Secret，再让 Helm 引用 `existingSecret`。

## 框架无法替代

WAF/API 网关、网络分区、主机/容器/数据库/中间件加固、堡垒机、统一身份治理、KMS/HSM、备份容灾、恶意文件检测、DLP、SAST/SCA/DAST/IAST、SOC/SIEM、等保定级备案测评、密评及组织制度仍属于具体系统和运行环境责任。

## 出站 TLS 与企业 CA

Redis、RocketMQ、Kafka Streaming、Elasticsearch/OpenSearch、S3-compatible 客户端均执行 TLS 1.2+、企业 CA、客户端证书/私钥和 ServerName 策略。RocketMQ 官方 Go SDK 的默认连接未直接采用，脚手架注入受控 gRPC TLS 连接以禁止跳过证书验证。框架**不提供 `InsecureSkipVerify` 配置**。生产内网如使用自建 CA，应挂载 CA/客户端证书 Secret 并配置对应 `*_TLS_*` 环境变量。数据库连接的 TLS/证书参数由 PostgreSQL/MySQL/OceanBase 驱动 DSN 管理。
