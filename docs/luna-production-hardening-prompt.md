# Luna 开发主提示词：Velora 生产/金融级整改

下面内容可直接完整复制给 Luna。建议每完成一个 Wave 后检查 diff 和验收结果，再回复“继续下一 Wave”，不要把全部整改压进一个不可审查的大提交。

---

你是 Velora 项目的生产级整改实施工程师。请在当前仓库内直接完成代码、配置、测试和文档修改，不要只给建议。

## 一、先读取并遵守

开始前完整读取：

1. 仓库适用的全部 `AGENTS.md`。
2. `docs/production-financial-product-readiness-assessment.md`。
3. `README.md`、`docs/architecture.md`、`docs/ops-deploy.md`、`docs/ops-backup.md`。
4. 与当前 Wave 相关的源码、测试、迁移和 Compose 文件。

先检查 `git status`。已有改动属于用户，不得覆盖、回滚或格式化无关文件。使用小步、可验证的改动；禁止顺便重构无关代码。

## 二、已经确定的架构，不要重新讨论

### 1. Casdoor 与 OIDC

- Casdoor 是唯一身份提供方（IdP），不得修改 Casdoor 源码。
- Velora 只作为标准 OIDC Client / Relying Party：浏览器跳转 Casdoor，Casdoor 登录/MFA，回调一次性 code，Velora 后端换取并校验 ID Token/UserInfo，然后建立服务端 session。
- 生产环境禁止 ROPC/password grant，用户密码不得经过 Velora。
- Velora 不再作为下游应用的 OIDC Provider。当前 `server/internal/oidcprovider`、`/oidc/*` 和 `VELORA_OIDC` 属于待下线能力：第一步 feature flag 默认关闭，生产强制关闭；不得删除已执行数据库迁移。确认没有消费者后才可在后续 PR 删除运行时代码。
- 不使用 Casdoor 管理员 API 修改用户、密码或角色。用户资料、改密、MFA 管理跳转 Casdoor 的自助页面，地址通过配置提供。
- Casdoor claims 负责身份事实：`sub`、用户名、邮箱、角色、组、组织；Velora 负责本地应用访问策略。
- 标准 OIDC 下游应用直接对接 Casdoor，并由应用自身生成 state、nonce、PKCE。Velora 的 `launch_url` 只保存目标应用自己的登录发起 URL或首页，不得替目标应用拼 authorize URL、state 或 verifier。
- 老系统使用 ForwardAuth。ForwardAuth 必须绑定目标 app ID/Host，并在真实授权边界执行 `CanAccess`；隐藏卡片不等于授权。

### 2. 对象存储

- 采用 vendor-neutral `ObjectStore` 接口和一个 S3-compatible adapter。
- 通过 endpoint、region、bucket、prefix、path-style、TLS、access key/secret、session token、SSE mode/key ID 配置兼容 MinIO、腾讯云 COS 和其他 S3 兼容服务。
- 不为 MinIO、COS 复制两套业务实现；厂商差异放在配置和 capability detection。
- 生产配置不得提供默认凭据；必须支持 checksum、失败重试、超时、multipart 大对象、私有 ACL/策略和可选不可变保留能力。
- MinIO 用于本地集成测试；COS 通过配置契约测试和预发布 smoke test 验证。

### 3. KMS/HSM 与国密

- 建立可插拔 `CryptoProvider`，业务代码不能直接依赖具体厂商 SDK。
- 密文 envelope 至少包含：`version`、`provider`、`algorithm`、`keyId`、`nonce`、`ciphertext`、`createdAt`。
- 支持 key version、双 key 解密、后台 rewrap/re-encrypt 和可回滚轮换。
- `standard` 能力可使用成熟实现的 AES-256-GCM；`gm` 能力面向 SM2/SM3/SM4，但必须调用经过审查的密码库、KMS/HSM、PKCS#11 或厂商服务，禁止自行实现密码算法。
- 在厂商尚未确定时，先实现接口、provider registry、capability、配置校验、fake provider 测试和迁移数据结构；配置 `CRYPTO_PROVIDER=gm` 但没有实际 adapter 时必须启动失败，不得假装已经完成国密合规。
- Velora 不再签发 OIDC Token，所以不要为了“国密”给 OIDC 发明非标准 `alg`。国密先覆盖邮件凭据、备份、对象存储和审计签名；TLCP/商密产品认证在部署方案确定后单独验收。

## 三、按 Wave 实施

一次只完成一个 Wave。每个 Wave 完成后运行全部验收命令，报告修改文件、测试结果、风险、数据库/配置迁移和回滚方法，然后停止等待复核。

### Wave 1：生产部署与供应链 P0

目标：先让生产配置不会直接暴露或使用默认秘密。

1. 建立独立的生产 Compose，不再依赖根开发 Compose 的多值合并；如果保留 overlay，必须通过最终 `docker compose config` 证明已清除继承端口。
2. 生产公网只发布 Web/TLS 网关 443；80 仅用于明确的 HTTPS 跳转。PostgreSQL、Redis、server、Casdoor、Prometheus、Grafana 不发布 host port，只进入隔离内部网络。
3. 删除生产默认 `postgres/postgres`、`velora/velora`、`admin/admin`；所有 Secret 使用无默认值的必填环境变量或 secret file。
4. 生产禁用 Casdoor `initData`，数据库使用独立最小权限账号。
5. `PUBLIC_BASE_URL`、Casdoor issuer/redirect、Redis、mail key 等生产配置缺失、HTTP 或 localhost 时启动失败。
6. 升级 Go 至至少 1.25.13，并升级 `x/text>=0.39.0`、`x/net>=0.55.0`、`pgx>=5.9.2`、`go-jose>=4.1.4`、`quic-go>=0.59.1` 或能消除当前报告漏洞的兼容版本。
7. Docker 构建禁止 `pnpm install --frozen-lockfile || pnpm install` 回退；固定可复现依赖。
8. 增加生产配置静态检查测试，验证最终端口、默认凭据和必填配置。

Wave 1 验收：

- `make test`
- `cd server && go test -race ./... && go vet ./... && govulncheck ./...`
- `cd web && pnpm audit --prod --registry=https://registry.npmjs.org`
- 使用 dummy secret 执行生产 `docker compose config`，输出中只能出现批准的 80/443 published ports，不得出现默认密码。
- `git diff --check`

### Wave 2：Casdoor 标准 OIDC 登录

1. 增加明确的 `AUTH_MODE=oidc`；生产只允许该模式，password/ROPC 仅可在显式 development 测试模式启用。
2. 登录页主操作跳转 `/api/v1/auth/oidc/login`；生产不渲染密码输入框。
3. 登录发起时创建服务端一次性 login transaction：保存 transaction hash、PKCE verifier、nonce、redirect、过期时间、consumed 状态；浏览器设置随机 HttpOnly、Secure、SameSite=Lax transaction cookie。
4. callback 必须同时验证 state、transaction cookie、issuer、nonce、PKCE、redirect 和未消费状态；成功或失败都使 transaction 失效。跨浏览器转交 callback 必须失败。
5. redirect 仅允许站内相对路径；禁止 scheme-relative、反斜杠和外部 URL。
6. 用 OIDC claims 建立服务端 session；记录 claims 刷新时间。优先使用标准 refresh token 定期刷新；若 Casdoor 不返回 refresh token，则生产 session TTL 不得超过一小时并要求重新认证，不能使用七天陈旧权限。
7. 移除生产对 Casdoor admin username/password 的依赖。Velora 用户中心的改密、MFA、资料编辑改为跳转 `CASDOOR_ACCOUNT_URL`。
8. logout 撤销 Velora session，并按 Casdoor discovery 支持情况执行标准 end-session；不支持时至少清理本地 session。

必须补测试：正常登录、state 篡改、跨浏览器 callback、nonce 错误、PKCE 错误、callback 重放、外部 redirect、session 过期和 logout。

### Wave 3：关闭 Velora OIDC Provider 并修正应用启动

1. 新增 `VELORA_OIDC_PROVIDER_ENABLED=false`，生产环境若为 true 直接启动失败。
2. flag 关闭时不注册 `/oidc/*`，不生成 signing key，不创建 OIDC client/token。
3. 管理后台不允许新建 `VELORA_OIDC` 应用；存量记录显示“待迁移”，不能静默继续使用。
4. 不删除 `0004_oidc_provider.sql` 等已执行迁移；数据库表保留，待消费者盘点和数据保留审批后另行处理。
5. 将普通 `OIDC` 应用的 `launch_url` 定义为目标应用自己的登录发起 URL/首页；删除 Velora 拼装 Casdoor authorize/state 的代码。
6. 对 launch URL 只允许管理员配置的 HTTPS 地址；开发环境可显式允许 loopback HTTP。
7. 更新 README、架构文档和后台字段文案，不能再声称 Velora 是 OIDC Provider。

### Wave 4：真实授权边界与凭据撤销

1. ForwardAuth 端点必须绑定目标 Application。推荐内部路由包含 app ID，并由网关根据预登记 Host 设置；直接客户端头部不得成为可信 app ID。
2. ForwardAuth 加载启用中的应用并执行统一 `CanAccess(user, app)`；拒绝禁用、未知、未授权应用。
3. 网关必须删除外部传入的身份/app 头，只注入后端验证后的头。
4. 建立统一 `CredentialRevocationService`：账户停用、全部下线、身份安全事件时撤销 server session、相关 refresh token/本地凭据并写可靠审计。
5. 由于生产不再使用自建 OIDC Provider，旧 `oidc_tokens` 只处理历史清理，不再产生新数据。
6. 关键权限变化使用权限版本或短期 claims 刷新；不得继续信任 168 小时快照。

必须补直接绕过测试：隐藏应用后直访、未授权 app ID、伪造 ForwardAuth header、禁用应用、角色撤销后旧会话。

### Wave 5：应用安全快速修复

1. 每个限流策略使用真实独立阈值；登录同时按 IP 和规范化账号限流。生产 Redis 故障时登录等敏感端点安全失败，普通读请求可按批准策略降级。
2. 对全局和登录请求使用 `http.MaxBytesReader`；限制 username、password、token、URL 和 JSON 字段长度。
3. 内存 lockout/limiter 必须有 TTL 清理、最大基数和淘汰；生产强制 Redis。
4. Todo URL 服务端和前端仅允许批准的 HTTPS scheme/host。
5. 健康检查和 IMAP 使用统一 EgressPolicy：解析全部 A/AAAA、阻止 loopback/private/link-local/metadata/unspecified/multicast、固定已验证 IP、每个重定向重新校验或禁止重定向。
6. IMAP 只允许批准 provider/端口；自定义 provider 需要管理员批准并有并发/速率配额。
7. 邮件正文拉取在下载前检查 RFC822.SIZE，并实施流式总量限制。
8. 邮件远程内容关闭时移除所有 URL-bearing CSS/style/src/srcset；优先 sandboxed iframe + 独立 CSP。

### Wave 6：数据库、健康和可观测性

1. 迁移 advisory lock 必须在同一 transaction/session 持有；更推荐独立 release migration job，应用实例不并发执行迁移。
2. liveness 只表示进程活着；readiness 检查 PostgreSQL、Redis 及必要身份依赖。Nginx `/healthz` 不得固定返回 200 冒充整体健康。
3. 增加按 method/route/status 的 HTTP 指标、依赖延迟、限流、claims 刷新、备份和审计指标。
4. 修正错误的 5xx 告警，增加通知路由、证书/磁盘/数据库容量/备份失败/Redis/Casdoor 告警。
5. `/metrics` 仅内部可访问。

### Wave 7：ObjectStore 与 CryptoProvider

先实现基础设施接口，不要一次把所有业务迁入。

建议接口能力：

```go
type ObjectStore interface {
    Put(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) (ObjectInfo, error)
    Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
    Head(ctx context.Context, key string) (ObjectInfo, error)
    Delete(ctx context.Context, key string) error
    Capabilities(ctx context.Context) (Capabilities, error)
}
```

要求：

- key 必须经过 prefix/tenant 规范化，禁止 `..`、反斜杠和越界。
- Put 支持 SHA-256 checksum、multipart、context cancellation、重试分类和幂等 object key。
- S3 adapter 使用成熟 SDK，支持 custom endpoint 和 path-style；所有凭据从 Secret 注入。
- 提供 `velora storage-check` 或等价只读/临时对象 smoke test。
- MinIO 集成测试放在显式 test profile，不进入生产默认编排。
- COS 不写专用业务分支；通过 endpoint/region/virtual-host 配置和 smoke test 验证。

建议密码接口：

```go
type CryptoProvider interface {
    Encrypt(ctx context.Context, purpose string, plaintext, aad []byte) (Envelope, error)
    Decrypt(ctx context.Context, purpose string, envelope Envelope, aad []byte) ([]byte, error)
    Rewrap(ctx context.Context, purpose string, envelope Envelope) (Envelope, error)
    Capabilities(ctx context.Context) (CryptoCapabilities, error)
}
```

要求：

- 先迁移邮件凭据；兼容解密旧 AES-GCM 数据，读取后惰性 rewrap 到新 envelope。
- 不允许静默丢失旧密钥导致用户重新绑定。
- fake provider 测试 key rotation、旧 key 解密、AAD 错误、密文篡改、未知 algorithm/keyId。
- `gm` provider 尚无实际 KMS/HSM adapter 时必须明确返回配置错误；不得以自写 SM 算法填空。

### Wave 8：备份、恢复与审计

1. 同一恢复点覆盖 Velora 和 Casdoor 两个数据库，支持 PostgreSQL PITR。
2. 备份默认 `umask 077`，先压缩/加密/签名再上传 ObjectStore；上传失败、checksum 不一致或 KMS 失败必须返回非零。
3. 对象使用私有策略、SSE/KMS（如果 provider 支持）、不可变保留/版本控制能力；能力不足必须显式告警或阻断生产策略。
4. 恢复不得吞错误，不得在容器内引用不存在的宿主路径；优先恢复到新环境并自动验数。
5. 审计高风险业务与 outbox 同事务；使用 CryptoProvider 签名/HMAC，定期将链头锚定到对象存储 WORM/SIEM。
6. 修复 `VerifyChain(startID)` 前驱和归档边界；归档加密、签名并保存 manifest/checksum。
7. 提供可自动执行的恢复演练和报告模板。

### Wave 9：CI、产品和发布门禁

1. CI 必须运行 test、race、vet、前端 lint/test/build、govulncheck、依赖 audit、secret、SAST、容器/IaC 扫描、SBOM 和许可证检查。
2. 登录、应用启动、ForwardAuth、权限拒绝、Todo、邮件和管理员配置建立 E2E。
3. 修复登录 label、嵌套交互控件、502 技术错误文案；完成 WCAG 2.2 AA 自动与人工检查。
4. 路由懒加载并建立 Web Vitals/Bundle budget。
5. 所有 P0/P1 验收通过后更新 `docs/production-financial-product-readiness-assessment.md` 状态，不得直接删除历史问题。

## 四、通用完成标准

每个 Wave 的最终答复必须包含：

1. 已完成结果，不要只描述过程。
2. 修改文件清单及关键行。
3. 新增/修改的配置、数据库迁移和兼容策略。
4. 执行过的命令及真实结果；未执行必须说明原因。
5. 安全反例测试结果。
6. 已知剩余风险、回滚步骤和下一 Wave 建议。

禁止事项：

- 不修改 Casdoor 源码。
- 不保留生产 ROPC 作为“临时备用”。
- 不继续完善 Velora 自建 OIDC Provider。
- 不删除或改写已发布迁移文件。
- 不自研 SM2/SM3/SM4、JWT、TLS、S3 签名或密码协议。
- 不提交真实 Secret、证书、Token 或云账号。
- 不用测试通过掩盖部署、协议、恢复和权限证据缺失。

现在开始时只执行 **Wave 1**。Wave 1 全部验收完成后停止并汇报，等待我确认再进入 Wave 2。

---

后续继续提示词：

> 复核当前工作树和上一 Wave 的验收证据，修复遗留失败后，按照 `docs/luna-production-hardening-prompt.md` 执行下一个尚未完成的 Wave。一次只完成一个 Wave，满足其全部测试、配置、兼容和回滚要求后停止汇报。
