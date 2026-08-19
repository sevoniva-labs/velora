# Velora 企业化生产实施方案

> 目标定位：**Velora = 企业门户入口 + 统一登录入口 + SSO 登录终点；Casdoor 只作为后端身份源，完全隐藏在 Velora 之后，不做任何修改。**
>
> 本文档是面向工程实施的总方案，覆盖从现状到生产可用的全部阶段（Phase A–D），每阶段含：目标、任务清单、技术设计、验收标准、工作量。配套代码审查见 `architecture.md`，品牌规范见 `brand.md`。

---

## 0. 现状摘要（评估结论）

| 维度 | 完成度 | 关键结论 |
| --- | --- | --- |
| 门户/工作台功能 | 8/10 | Phase 1 目标基本达成，待办/邮件已产品化 |
| 认证安全基线 | 6/10 | 协议层扎实（PKCE/state/nonce/CSRF/防 Open Redirect），但会话不可吊销、无 MFA |
| **统一登录入口（核心目标）** | **2/10** | **Velora 是 OIDC 客户端（RP），不是 OIDC 服务端（IdP/Provider）** |
| 高可用与部署 | 2/10 | 单点 HTTP 部署，无 TLS、无备份、无多副本 |
| 可观测性/运维 | 3/10 | 有结构化日志 + healthz，无 metrics/告警/集中日志 |
| 数据与性能 | 4/10 | 内存过滤分页、ILIKE 全表扫、无备份恢复演练 |
| 工程化 | 3/10 | 无 CI/CD，测试覆盖薄（Go 7 个测试文件 / Web 4 个） |
| **综合** | **≈ 4/10** | 内部工具可用；企业统一入口差一个 IdP 模块 + 一套基础设施 |

**最重要的架构事实**：当前 `server/internal/application/launch.go` 的 `OIDCLaunchProvider` 把应用直接指向 **Casdoor** 的 authorize 端点（`{issuer}/login/oauth/authorize`），用户点击 OIDC 应用会看到 **Casdoor 登录页**，而不是 Velora。这与"所有登录都走 Velora、Casdoor 藏在后面"的目标**根本冲突**。要达成目标，Velora 必须新增 **OIDC Provider（IdP）能力**——这是 Phase B 的核心，也是本方案与"修修补补"的本质区别。

---

## 1. 目标架构

```text
                         用户
                          │
                          ▼
        ┌─────────────────────────────────┐
        │            Velora               │
        │  ┌───────────────────────────┐  │
        │  │  门户 / 工作台 / 待办/邮件 │  │
        │  ├───────────────────────────┤  │
        │  │  OIDC Provider (IdP)      │  │  ← 新增（Phase B）
        │  │  /authorize /token        │  │
        │  │  /userinfo /jwks          │  │
        │  │  登录页（Velora 品牌）      │  │
        │  └────────────┬──────────────┘  │
        └───────────────┼─────────────────┘
                        │ 仅 Velora 内部调用（OIDC/ROPC）
                        ▼
                    ┌──────────┐
                    │ Casdoor  │  身份源（不改、不直连库、用户不可见）
                    └──────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
     App A             App B             App C
   （对接 Velora SSO）（对接 Velora SSO）（对接 Velora SSO）
```

关键转变：

- **现在**：应用 → 跳 Casdoor 登录 → 回应用。（用户面对 Casdoor）
- **目标**：应用 → 跳 Velora 登录 → Velora 静默用已有会话放行（未登录则展示 Velora 登录页）→ 回应用。（用户永远面对 Velora；Casdoor 仅由 Velora 通过 OIDC/ROPC 内部消费）

---

## 2. 实施总览

| 阶段 | 名称 | 周期（1 人全职） | 上线硬门槛 | 目标 |
| --- | --- | --- | --- | --- |
| **Phase A** | 基础设施地基 | 1–2 周 | ✅ 是 | TLS 全链路、DB 备份恢复、监控告警、CI、集中日志 |
| **Phase B** | OIDC Provider（核心架构） | 2–4 周 | ✅ 是 | Velora 成为登录终点，应用对接 Velora SSO；会话可吊销 |
| **Phase C** | 安全加固 | 2–3 周 | 建议 | MFA、登录风控、自助用户中心、审计强化 |
| **Phase D** | 持续增强 | 持续 | 否 | SAML/CAS/ForwardAuth、性能、待办鉴权、邮件增强、i18n |

依赖关系：`Phase A ⊂ Phase B 前提（TLS 是 OIDC issuer 校验要求）`；Phase C 依赖 Phase B 的会话改造；Phase D 可并行。

---

# Phase A — 基础设施地基（1–2 周）

## A1. 全链路 HTTPS

**现状**：`deployments/docker/nginx.conf` 仅 `listen 80`；Casdoor `origin: http://localhost:8443`；`CASDOOR_ISSUER=http://localhost:8443`。OIDC 生产强制 HTTPS（issuer 必须是 https，否则 `go-oidc` 的 issuer 严格校验与浏览器混合内容都会失败）。

**任务**：

1. 引入统一入口 Nginx（或网关）终止 TLS：
   - `velora.企业域.com` → Web 静态 + `/api` 反代到 server
   - `sso.velora.企业域.com`（Velora 登录/IdP 端点）+ `/casdoor/` 路径或独立子域反代到 Casdoor
2. 证书：企业内网用内网 CA + 受信根下发；公网用 Let's Encrypt（certbot + 自动续期）。
3. Casdoor 侧：`origin` 改为 `https://sso.企业域.com`（如走子域反代）；`CASDOOR_ISSUER` 同步改 https；`COOKIE_SECURE=true`。
4. compose 增加 `VELORA_EXTERNAL_URL` / `CASDOOR_ISSUER` 等生产环境变量，与开发环境（http://localhost）隔离。
5. HSTS 头（`Strict-Transport-Security`）加入 nginx 与后端安全头。

**验收**：浏览器地址栏全部 https；OIDC 登录/回调全链路无混合内容告警；`/api/v1/system/version` 经 https 可访问。

## A2. 数据库备份与恢复

**现状**：数据（应用/收藏/待办/邮件/审计）无任何备份策略。

**任务**：

1. `scripts/backup-db.sh`：`pg_dump`（或 `pg_dumpall`）按日全量 + 归档 WAL（PITR），输出到独立磁盘/对象存储（如 MinIO/S3），保留 30 天滚动。
2. `scripts/restore-db.sh` + 演练文档：每月一次恢复演练，验证"从备份恢复到新库"。
3. compose 增加可选 `backup` 服务（cron + 卷挂载），生产建议独立于应用主机。
4. 审计表（`audit_logs`）单独归档策略（见 C5），防止主库无限膨胀。

**验收**：可执行一键备份；有书面恢复演练记录；备份文件可异地访问。

## A3. 可观测性：Metrics / 告警 / 集中日志

**现状**：`middleware.go` 有 `observeRequest()` 埋点但无真正的 metrics 导出端点；日志仅 stdout（compose 下进 docker logs）。

**任务**：

1. 后端增加 `/metrics` 端点（Prometheus 文本格式），暴露：
   - HTTP 请求量/延迟/错误率（按 route、status 分桶）
   - 业务指标：登录成功/失败数、Launch 次数、审计写入失败数、邮件同步耗时/失败数、数据库连接池状态
   - Go runtime（goroutine、GC、heap）
2. 部署 Prometheus + Grafana：预置告警规则（5xx 率突增、登录失败激增、DB 不可达、磁盘水位、证书 30 天过期）。
3. 日志：`VELORA_ENV=production` 已输出 JSON（`main.go`），接入集中日志（Loki/ELK/云厂商 SLS 任选），按 `requestId` 关联请求全链路。
4. 告警通道：企业 IM webhook（钉钉/企微）或邮件。

**验收**：Grafana 面板可看到请求 QPS/延迟/错误率；配置的告警规则可触发通知；日志可按 requestId 检索。

## A4. CI/CD

**现状**：无任何 CI 配置。

**任务**：

1. GitHub Actions（或内网 GitLab CI）流水线，PR 必过门禁：
   - `make lint`（go vet + oxlint）
   - `make test`（go test ./... + vitest）
   - `pnpm build` + `go build`
2. 合并到 main 后自动构建镜像并推送私有 registry（`docker.m.daocloud.io` 仅作拉取加速，**推送**用企业 Harbor/云 registry）。
3. 环境管理：dev / staging / prod 三套 compose overlay（`docker-compose.override.yml` 或独立目录 `deployments/env/*`），密钥走环境变量/密钥管理，不进 git。

**验收**：PR 无法合入未通过检查的代码；main 变更自动出镜像；三环境一键部署脚本存在且可复现。

## A5. 配置与密钥管理

**现状**：`SESSION_SECRET`、`MAIL_CREDENTIAL_KEY`（开发缺省从 SESSION_SECRET 派生）、Casdoor admin 凭据均在 `.env` / compose 环境变量明文。

**任务**：

1. 生产引入密钥管理（Vault / 云 KMS / Docker Secrets 任选），密钥不落盘、不入 compose 明文。
2. `MAIL_CREDENTIAL_KEY` 生产必须显式配置为独立 base64 32 字节密钥（与 SESSION_SECRET 分离），杜绝派生。
3. 新增 `OIDC_SIGNING_KEY` / `OIDC_ROTATION_KEYS`（Phase B 需要，见 B5）。
4. 文档化密钥轮换流程（SESSION_SECRET 轮换会使所有会话失效，需选低峰窗口）。

**验收**：生产环境无明文密钥；密钥轮换流程文档化并演练过。

---

# Phase B — OIDC Provider（核心架构，2–4 周）

> 这是达成"Velora 是登录终点"的核心模块。Phase B 完成后：第三方应用只需把 Velora 配为 OIDC IdP（`issuer`、`client_id`、`client_secret`、回调），用户跳转登录时看到的是 **Velora 登录页**。

## B1. 总体设计

新增 `server/internal/oidcprovider` 领域包，实现标准 OIDC Provider 子集（Authorization Code + PKCE；后续可加 refresh token / client credentials）。

```text
用户/应用 ──► GET /oidc/.well-known/openid-configuration（发现）
             GET /oidc/authorize?client_id&redirect_uri&response_type=code&scope&state&code_challenge
                 ├─ 已登录（Velora 会话有效）→ 直接签发 code，302 回应用
                 └─ 未登录 → 302 到 Velora /login?redirect=/oidc/authorize?...（复用现有登录页）
             POST /oidc/token（code 换 access_token + id_token，校验 PKCE）
             GET  /oidc/userinfo（Bearer access_token）
             GET  /oidc/jwks（公钥列表，支持轮换）
             GET  /oidc/logout（可选，RP-initiated logout）
```

**注册表**：`oidc_clients`（每个第三方应用一个 client）：`client_id`、`client_secret_hash`（只存哈希）、`redirect_uris`（白名单，防 Open Redirect）、`grant_types`、`scopes`、`enabled`。

**与门户应用模型的关系**：第三方应用的 `client_id` 与门户 `applications` 表通过 `application_id` 外键关联（一个门户应用可挂多个 OIDC client？不，建议 1:1，管理员在"应用 → 接入类型 OIDC"处自动生成 client_id/secret 并展示给对接方）。

## B2. 数据模型（迁移 `0004_oidc_provider.sql`）

```sql
-- OIDC 客户端（第三方应用接入 Velora SSO）
CREATE TABLE oidc_clients (
    id               BIGSERIAL PRIMARY KEY,
    application_id   BIGINT REFERENCES applications(id) ON DELETE CASCADE,
    client_id        VARCHAR(128) NOT NULL UNIQUE,
    client_secret_hash VARCHAR(256) NOT NULL,       -- 只存哈希（bcrypt/argon2），页面仅展示一次
    redirect_uris    TEXT NOT NULL,                  -- JSON 数组，白名单
    grant_types      TEXT NOT NULL,                  -- JSON 数组：authorization_code, refresh_token
    scopes           TEXT NOT NULL DEFAULT '["openid","profile","email"]',
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 一次性授权码（短时，10 分钟）
CREATE TABLE oidc_auth_codes (
    id           BIGSERIAL PRIMARY KEY,
    code_hash    VARCHAR(128) NOT NULL UNIQUE,       -- 存哈希，防泄露
    client_id    VARCHAR(128) NOT NULL,
    user_id      VARCHAR(128) NOT NULL,
    redirect_uri VARCHAR(512) NOT NULL,
    scope        TEXT NOT NULL,
    code_challenge     VARCHAR(256) NOT NULL DEFAULT '',
    code_challenge_method VARCHAR(16) NOT NULL DEFAULT 'S256',
    expires_at   TIMESTAMPTZ NOT NULL,
    used         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 令牌（access/refresh；access 也可用 JWT 无状态，refresh 必须落库以便吊销）
CREATE TABLE oidc_tokens (
    id            BIGSERIAL PRIMARY KEY,
    client_id     VARCHAR(128) NOT NULL,
    user_id       VARCHAR(128) NOT NULL,
    access_jti    VARCHAR(128) NOT NULL UNIQUE,      -- JWT jti（吊销索引）
    refresh_hash  VARCHAR(128) NOT NULL UNIQUE,      -- refresh token 哈希
    scope         TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 签名密钥对（支持轮换；私钥加密存储或由 KMS 注入）
CREATE TABLE oidc_keys (
    kid          VARCHAR(64) PRIMARY KEY,
    alg          VARCHAR(16) NOT NULL DEFAULT 'RS256',
    public_pem   TEXT NOT NULL,
    private_pem  TEXT NOT NULL,                      -- 建议加密（AES-GCM，密钥来自 KMS/env）
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    not_before   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ
);
```

**会话改造（配套）**：Phase B 同时把无状态 HMAC 会话升级为**服务端会话**（见 C1 的 B 前置部分：`sessions` 表或 Redis），否则"已登录静默放行 authorize"无法支持吊销/强制下线。最低要求：authorize 校验会话有效性 + 支持立即吊销。

## B3. 端点契约

| 端点 | 方法 | 认证 | 说明 |
| --- | --- | --- | --- |
| `/oidc/.well-known/openid-configuration` | GET | 无 | issuer、authorization_endpoint、token_endpoint、userinfo_endpoint、jwks_uri、response_types_supported、code_challenge_methods_supported |
| `/oidc/authorize` | GET | Velora 会话 | 校验 client/redirect_uri/scope/state/PKCE challenge；已登录 → 生成 code 302 回应用；未登录 → 302 到 `/login?redirect=...` |
| `/oidc/token` | POST | client_id+secret（Basic 或 body） | code 换 token；校验 PKCE verifier、code 一次性、未过期 |
| `/oidc/userinfo` | GET | Bearer access_token | 返回 sub/preferred_username/email/display_name/roles 等 claims |
| `/oidc/jwks` | GET | 无 | 当前有效公钥列表（含轮换中的旧 key） |
| `/oidc/logout` | GET/POST | 可选 | RP-initiated logout：吊销 refresh、清 Velora 会话 |

**安全要求**（对齐现有标准）：
- code 与 refresh token 一律存哈希（SHA-256），数据库泄露不可用。
- redirect_uri 严格白名单匹配（完整 URL 前缀或精确匹配，拒绝 `\`、`//`、百分号编码绕过——复用 `auth/redirect_test.go` 的防护经验）。
- state 必须透传校验；PKCE 强制（S256，拒绝明文 verifier 或缺失）。
- access_token（JWT RS256）签名 key 与 id_token 同 key 或分 key 均可，`kid` 必须带。
- token 端点限流（复用 `server.go` 的 `rateLimit`，Phase C 升级为 Redis 分布式）。

## B4. 登录流程（两种模式）

**模式 1：Velora 会话已存在（静默 SSO）**
应用跳转 `/oidc/authorize` → 中间件校验 Velora 会话有效 → 签发 code → 302 回应用 → 应用换 token → 完成。**用户无感**。

**模式 2：未登录**
`/oidc/authorize` 302 到 Velora `/login?redirect=/oidc/authorize?<原参数>` → 用户看到 Velora 登录页（品牌统一）→ 登录成功（ROPC 代理 Casdoor 或授权码）→ 回跳 `/oidc/authorize?...` → 静默签发 code。

**Casdoor 的角色同步**：`/oidc/userinfo` 的 roles/groups claims 必须来自**本次 Velora 会话**（登录时从 Casdoor id_token/UserInfo 解析并存入服务端会话），不要实时查 Casdoor（避免每次 userinfo 都打 Casdoor；角色变更延迟与现有会话 TTL 权衡一致，文档化）。

## B5. 密钥与 Token 管理

- 密钥生成：`openssl genrsa 2048`（或 ECDSA P-256）→ 存入 `oidc_keys`（private_pem 加密）或由 KMS 注入。
- 轮换：每 90 天新增一对 key（新 kid），旧 key 保留 30 天用于验证存量 token；`jwks` 同时返回新旧公钥；`/.well-known` 与 `jwks_uri` 始终返回最新。
- access_token 生命周期：短（默认 1h，JWT 无状态，`exp` + `jti`）；refresh_token 长（默认 30d，落库可吊销）。
- 吊销：用户强制下线 / 改密 → 按 `user_id` 批量置 `revoked_at`；access_token 因无状态无法即时吊销，靠短 TTL + 黑名单（`jti` 布隆/Redis，可选）。

## B6. 与现有模块的整合

- **应用接入**：管理后台"应用 → 接入类型"增加 `VELORA_OIDC`（区别于现有 `OIDC`=Casdoor 直连）：保存时自动生成 client_id/secret，页面一次性展示 secret 与对接参数（issuer、authorize/token/userinfo/jwks 地址、示例 curl）。
- **Launch 行为变更**：`ssoType=VELORA_OIDC` 的应用，Launch 返回 `{type:"url", url:"{velora}/oidc/authorize?..."}`（`_blank` 或 `_self` 按配置）——**不再跳 Casdoor**。现有 `OIDC`（Casdoor 直连）类型保留兼容存量，新接入一律推荐 `VELORA_OIDC`。
- **审计**：`authorize` 成功/失败、`token` 签发、`userinfo` 访问均写审计（`ActionOidcAuthorize` / `ActionOidcToken`）。
- **登录页复用**：`/login?redirect=` 已支持站内相对路径回跳（`oidc.go` state 机制），扩展允许 `/oidc/authorize?...` 这类内部路径（在现有严格相对路径校验上增加白名单前缀）。
- **健康检查**：`/healthz` 增加 oidcprovider 自检（可选）。

## B7. 兼容与迁移

1. 存量 `OIDC`（Casdoor 直连）应用不动，功能保持；新应用用 `VELORA_OIDC`。
2. 迁移工具：提供管理脚本将存量 OIDC 应用批量切换为 Velora 直连（需各应用方在 Casdoor 侧删除 client、在 Velora 侧重建 client——**涉及应用方协调**，建议灰度：先切一个试点应用验证）。
3. 会话改造（服务端会话）采用平滑迁移：新会话写库，旧 HMAC 会话在 TTL 内兼容，到期自然失效。

## B8. 验收标准（Definition of Done）

- [ ] 用标准 OIDC 客户端（如 `oidc-client-ts` 或 Postman OIDC 流程）完成：发现 → authorize → 登录（含未登录跳 Velora 登录页）→ token → userinfo 全链路。
- [ ] PKCE 强制：缺 verifier 或 code_challenge 不匹配返回错误。
- [ ] redirect_uri 白名单外一律拒绝（含 `\`、`//`、编码绕过用例）。
- [ ] code 一次性：二次使用拒绝；code/refresh 过期拒绝；库中只存哈希。
- [ ] 已登录用户 authorize 无感放行；登出后立即拒绝。
- [ ] 密钥轮换演练：轮换后旧 token 仍可验（宽限期），新 token 用新 key。
- [ ] 审计出现 authorize/token 记录；metrics 出现 OIDC 请求量。
- [ ] 单元测试覆盖：state/PKCE/redirect 校验/token 签发/吊销（对齐现有 `oidc_state_test.go` 风格）。

---

# Phase C — 安全加固（2–3 周）

## C1. 服务端会话与吊销（Phase B 前置）

- 新增 `sessions` 表（或 Redis）：`session_id`（随机 256bit）、`user_id`、`roles` 快照、`expires_at`、`revoked_at`、`created_ip`、`user_agent`、`last_seen_at`。
- Cookie 只存 `session_id`（随机值），服务端查表校验；**吊销 = 置 revoked_at**（立即生效，解决现有"被盗 Cookie 无法作废"）。
- 能力：强制下线（按用户）、设备列表管理、改密后全端下线、会话滑动过期（可选）。
- 兼容：Phase B 期间新旧会话双轨，新写旧读；TTL 后旧机制自然退出。

## C2. MFA（多因素认证）

**决策点（需技术验证）**：Casdoor 支持 TOTP/WebAuthn MFA，但 Velora ROPC 代理无法透传 MFA 挑战。两条路线：

- **路线 1（推荐，体验统一）**：登录改为**授权码模式**（Velora 302 到 Casdoor authorize，Casdoor 登录页处理 MFA），但把 Casdoor 登录页**反代到 Velora 域名下**（`/casdoor/` 路径，nginx 反代 + 品牌配置），视觉上仍在 Velora 域内。代价：登录页是 Casdoor 渲染（可配 logo/品牌色），Velora 登录页变成纯跳转。适合对 MFA 是硬性合规要求的企业。
- **路线 2（保留 Velora 登录页）**：Velora 自己实现 TOTP 二步验证（用户 MFA secret 从 Casdoor 用户属性读取或 Velora 侧维护）。工作量大且与 Casdoor 身份体系有耦合风险。
- **折中**：默认关 MFA 用 Velora 登录页直登；开启 MFA 的组织自动走授权码（路线 1）。

**验收**：开启 MFA 的账号无法仅凭密码登录；二次验证失败有审计与限流。

## C3. 登录风控

- **分布式限流**：`server.go` 内存 `rateLimit` 替换为 Redis 滑动窗口（compose 已有可选 redis profile）。按 IP、账号、client_id 多维度。
- **账户锁定**：连续失败 N 次（如 5 次/15 分钟）锁定账号（Redis 计数 + 落库审计），支持管理员解锁。
- **异常检测（P2 可选）**：登录 IP 与历史差异、UA 突变 → 二次验证或告警。

## C4. 自助用户中心（封装 Casdoor，用户不可见）

- Velora 内实现：修改密码、忘记密码（邮件/短信验证码）、绑定/解绑手机与邮箱、查看登录设备并下线。
- 实现方式：后端调 **Casdoor 管理 API**（`casdoor/sync.go` 已有 admin 凭据读取模式，扩展为写操作需要谨慎——**最小权限**：仅本组织用户自助字段，禁用全局管理端点）。
- 若 Casdoor 管理 API 权限过大，折中：改密/找回跳转 Casdoor 的对应页面（反代到 Velora 域），自助信息展示留在 Velora。

## C5. 审计强化

- **防篡改**：审计表增加 `prev_hash` 链式哈希（每条记录 `hash = H(prev_hash || 内容)`），检测断链即告警；或定期导出归档到对象存储（append-only）。
- **登录失败审计**：ROPC 失败、authorize 失败、MFA 失败均记录（含 IP/UA）。
- **保留策略**：在线 180 天，归档 3 年（脚本 + 存储生命周期）。
- **审计查询**：管理后台审计页支持时间范围与导出 CSV（合规取证）。

---

# Phase D — 持续增强（并行/持续）

## D1. 应用接入类型扩展

- 复用 `LaunchProvider` 扩展点实现 **SAML**（SP 模式，Velora 作为 SP 对接应用 IdP 或作为 IdP 签发 SAML Assertion）、**CAS**、**Forward Auth**（Nginx/APISIX 校验回调）。Phase 2 范围，先做 Forward Auth（成本低、价值高：给非 OIDC 老系统套 SSO）。

## D2. 性能

- `service.go ListPublic` 的"全量查库 + 内存过滤 + 内存分页"改为 **SQL 级策略过滤**：`application_access_policies` 反范式（每用户可见集物化或 join 过滤），分页下推数据库。
- 搜索 `ILIKE '%kw%'` 增加 `pg_trgm` GIN 索引；分页用 keyset（`WHERE id > ?`）替代 OFFSET。
- 应用列表/收藏/最近使用接口增加本地缓存（短 TTL，TTL 内可容忍的过期数据）。

## D3. 待办推送独立鉴权

- `POST /api/v1/todos` 目前复用管理员会话。新增 **service account**：`integration_tokens` 表（token 哈希 + 归属系统 + 权限 scope），对接方用 `Authorization: Bearer` 推送，与用户会话解耦；审计记录来源系统。

## D4. 邮件模块增强

- IMAP IDLE 实时推送（替代 10 分钟轮询补偿）；多实例互斥（DB lease / Redis lock，避免重复拉取）。
- 附件下载、SMTP 回复/转发（Phase 3）。

## D5. i18n 与合规

- 文案抽离（现有 `labels.ts` + 后端 `errs` 中文 message）→ i18n 资源；`Accept-Language` 切换。
- 用户数据导出（GDPR/个保法）：管理员可导出指定用户的全量数据（应用访问/收藏/待办/邮件元数据）。

---

# 3. 排期与人力

| 阶段 | 周期 | 里程碑 | 依赖 |
| --- | --- | --- | --- |
| Phase A | 1–2 周 | 生产可部署（TLS/备份/监控/CI） | 无 |
| Phase B | 2–4 周 | 应用对接 Velora SSO 全链路打通 | Phase A（TLS） |
| Phase C | 2–3 周 | 会话可吊销、MFA、风控、审计强化 | Phase B |
| Phase D | 持续 | 接入类型/性能/鉴权增强 | 并行 |

- 总投入：**约 2–3 个月（1 人全职）** 到达企业生产基线（A+B+C）；Phase D 视业务优先级滚动。
- 建议并行：A4 CI 与 D2 性能可提前；B 是单人不可压缩的核心路径。
- 建议先做一个**试点应用**走通 Velora SSO（B 完成后），验证对接体验再批量迁移。

---

# 4. 风险与开放问题

| 风险/问题 | 影响 | 缓解 |
| --- | --- | --- |
| 存量 OIDC 应用迁移需应用方协调（Casdoor client → Velora client） | 中 | 灰度切换、试点先行、双轨兼容期 |
| MFA 体验：Velora 登录页 vs Casdoor 授权码页二选一 | 高（合规场景） | C2 路线 1（Casdoor 页反代到 Velora 域）需先做 POC 验证品牌一致性 |
| 服务端会话引入状态存储（Redis/DB），架构从无状态变有状态 | 中 | Redis 高可用 + 会话表降级；Phase B 平滑迁移 |
| Casdoor 管理 API 用于自助中心（写操作）的权限边界 | 高 | 最小权限账户、操作审计、必要时跳转 Casdoor 页面 |
| 邮件多实例重复拉取 | 低 | D4 DB lease / Redis lock |
| 审计防篡改链增加写放大 | 低 | 异步批量 + 独立归档 |

---

# 5. 涉及文件清单（增量）

```text
server/
├── migrations/0004_oidc_provider.sql        # oidc_clients / auth_codes / tokens / keys
├── migrations/0005_sessions.sql             # 服务端会话
├── internal/oidcprovider/                   # 新增：IdP 核心（well-known/authorize/token/userinfo/jwks/logout + 密钥轮换 + token 服务）
├── internal/auth/session.go                 # 改造：session_id 查表 + 吊销
├── internal/application/launch.go           # 新增 VELORA_OIDC Provider；保留存量 OIDC
├── internal/application/model.go            # ssoType 枚举 + oidc client 字段
├── internal/todo/handler.go                 # D3：service account 鉴权
├── internal/platform/httpserver/server.go   # 挂载 OIDC 路由、metrics 路由、Redis 限流
├── internal/platform/httpserver/metrics.go  # A3：/metrics
└── internal/casdoor/                        # C4：自助中心 API 封装（最小权限）
web/
├── src/pages/admin/Applications.tsx         # 接入类型 VELORA_OIDC + client 生成展示
├── src/pages/admin/Settings.tsx             # MFA/安全设置
├── src/pages/UserCenter.tsx（新增）          # C4 自助中心
└── src/api/api.ts                           # 新端点封装
deployments/
├── docker/nginx.conf                        # A1：TLS/443/HSTS/反代 /casdoor/
├── env/dev|staging|prod/                    # A4：三环境 overlay
└── compose/backup                           # A2：备份服务
scripts/
├── backup-db.sh / restore-db.sh             # A2
├── rotate-oidc-keys.sh                      # B5
└── migrate-oidc-apps.sh                     # B7 存量迁移
.github/workflows/ci.yml                     # A4
docs/
├── production-plan.md                       # 本文档
├── oidc-provider-api.md（建议新增）          # B3 端点契约细化 + 对接方手册
└── ops-runbook.md（建议新增）                # 备份/恢复/密钥轮换/故障演练手册
```

---

## 结语

Velora 的产品形态（门户 + 工作台 + 待办/邮件）已经过了"能不能用"的关口，剩下的是"**能不能作为企业登录基座**"的工程问题。**Phase A + B 是上线硬门槛**：A 解决"敢不敢部署"，B 解决"是不是登录终点"。本文档按依赖排序给出可执行路径，建议按 Phase A → B → C 顺序推进，Phase D 滚动并行。
