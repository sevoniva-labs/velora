# Velora 架构与设计决策

本文档记录 Velora 第一阶段的关键架构决策与边界，供后续开发者快速理解。

## 1. 身份边界（最重要）

> **Casdoor manages identity. Velora manages the workspace.**

- Casdoor：唯一身份 / IAM / SSO（外部 OIDC Provider）。
- Velora：OIDC Client/Relying Party、门户 / 工作台 / 应用枢纽。
- Velora 不签发 OIDC Token，不注册 `/oidc/*`，也不接收 Casdoor 密码。
- Velora **永不**直连 Casdoor 数据库；只通过 OIDC（登录）、Casdoor API（后续扩展）消费身份。
- 数据库隔离：同一 PostgreSQL Server，独立 database `casdoor` / `velora`。

## 2. 登录流（OIDC Authorization Code + PKCE）

```text
访问 Velora → GET /api/v1/me 401 → 登录按钮
→ GET /api/v1/auth/federated/oidc/casdoor/begin
   （服务端保存一次性 state、PKCE verifier、nonce、组织和 5 分钟 TTL）
→ 设置 HttpOnly/Secure/SameSite=Lax 交易 cookie
→ 302 Casdoor authorize?code_challenge=...&code_challenge_method=S256
→ Casdoor 认证 → 302 回调 /api/v1/auth/federated/oidc/casdoor/callback?code&state
→ 校验 state、交易 cookie、issuer、nonce、PKCE 和一次性消费
→ code 换 token，校验 ID Token，建立可撤销服务端 session
```

安全要点：

- state、nonce、PKCE verifier 只保存在服务端短时交易中；
- nonce 校验防重放，state 使用 CompareAndDelete 一次性消费；
- redirect 仅允许站内相对路径（防 Open Redirect）；
- Session Cookie：`HttpOnly` + `Secure`（配置）+ `SameSite`，`SESSION_SECRET` 签名。

## 3. 请求/响应约定

统一响应（所有 `/api/v1` 端点）：

```json
{ "code": "000000", "message": "success", "data": {}, "requestId": "..." }
```

错误码模块（见 `internal/platform/errs`）：

```text
000000 SUCCESS
A01xxx AUTH        认证 / OIDC / Session
A02xxx APPLICATION 应用领域
A03xxx PERMISSION  访问控制
A04xxx PORTAL      分类 / 标签 / 收藏 / 设置
A05xxx SYSTEM      系统 / 参数 / 数据库
```

禁止对外返回：SQL 错误、堆栈、内部路径、密钥。

## 4. 应用可见性与访问控制

- 数据模型：`application_access_policies`（EVERYONE / ORGANIZATION / ROLE / GROUP / USER，OR 语义，空策略 = 全员可见）。
- 前端：列表按策略过滤展示（隐藏无权应用）。
- 后端：`CanAccess` 在 List / Get / Launch / Favorite 全链路强制校验，直接调 API 返回 403。
- ForwardAuth：老系统调用 `GET /api/v1/auth/forward/{application_id}`，后端从可信路由参数加载应用并执行同一 `CanAccess`；网关必须剥离入站 `X-Velora-*` 头，只转发后端响应头，不能把客户端 app-id/Host 当授权依据。
- 管理员：持有 `VELORA_ADMIN_ROLE`（默认 `velora_admin`，来自 Casdoor 角色）可管理应用/分类/标签/策略/审计。

## 5. Launch 安全

- 不接受客户端 URL（`POST /applications/:id/launch`，服务端读库）。
- `LaunchProvider`：`URL` 仅允许管理员配置的 HTTPS 地址；`OIDC` 的 `launch_url` 是目标应用自己的登录发起 URL/首页，不由 Velora 拼装 authorize、state 或 verifier。
- `SAML` / `CAS` / `FORWARD_AUTH` 仅保留模型与扩展点，未实现时明确报错（不做假实现）。

## 6. 模块边界

```text
cmd/server|worker|migrate
internal/domain       业务不变量（identity / portal / approval）
internal/app           应用服务与事务边界
internal/adapters      PostgreSQL repository 与 Kratos transport
internal/platform      config / cache / database / storage / crypto / authn / authz
internal/bootstrap     依赖组装、迁移和健康检查
```

## 7. 前端复用 Spectra 的边界

- 复用：`theme/tokens.ts`（Design Token）、`index.css` 样式体系（前缀改造为 `velora-`）、`api/client.ts` 请求封装（适配 Velora 统一返回）、`RequireAuth` / `useMe`、顶栏布局结构、工程配置（Vite / tsconfig / oxlint）。
- 重写：业务页面（门户/管理后台）、领域 API、菜单内容、标签映射。
- 禁止：复制 Spectra DevOps 业务逻辑；运行时依赖 Spectra 本地路径；修改 Spectra 目录。

## 8. 数据库迁移

- `server/migrations/*.sql`，按文件名顺序执行，`schema_migrations` 记录，幂等。
- 通过 `//go:embed` 嵌入二进制（`migrations` 包），`velora serve` 启动时自动迁移。

## 9. 已知权衡（Phase 1 明示）

- **会话可撤销**：会话记录位于 Velora 数据库，角色/停用/全部下线可立即撤销。
- **服务端 OIDC 交易**：state、nonce、PKCE verifier 和交易 cookie 均短时、一次性、失败即失效，不出现在 URL 中。
- **Portal 访问控制**：列表、详情、启动、收藏均调用统一 `CanAccess`；管理员策略修改写可靠审计。

## 10. 中国大陆开发环境

- Go：`GOPROXY=https://goproxy.cn,direct`（`scripts/bootstrap-cn.sh` 提供，不修改全局配置）。
- pnpm：`web/.npmrc` 项目级 `registry=https://registry.npmmirror.com`。
- Docker：`DOCKER_REGISTRY` ARG 支持 `docker.m.daocloud.io` ↔ `docker.io` 切换；镜像锁定稳定版本。

## 11. 待办中心（外部系统集成 API）

> 当前 Wave 1 基座尚未提供这些 `/todos` 运行时接口；生产前端不展示待办入口。以下为后续接入契约，
> 接入前必须补齐后端权限、幂等、分页、审计和 E2E，不得把前端空数组当作已上线能力。

待办中心面向"统一待办"场景：其他业务系统（OA / 审批 / 运维工单等）通过 API 把待办推送到门户，用户在首页统一查看处理、点击跳回来源系统单据页。

- 表 `todos`（`0002_todos.sql`）：`(source_system, source_id, user_id)` 唯一索引作为幂等键；`priority` 取 `urgent|high|mid|low`；`status` 取 `open|done`。
- `GET /api/v1/todos?status=open|done|all&limit=N`：当前用户待办（按到期时间升序、无到期排后），返回 `{ items, openCount }`。
- `POST /api/v1/todos`：**管理员限定**（供对接方使用）。按幂等键 upsert——同一来源单据重复推送只更新 `title/sourceLabel/priority/url/dueAt`，不产生重复待办，且不会把已完成待办重置回 open。请求体：`{ userId, title, sourceSystem, sourceLabel, sourceId, priority, url, dueAt }`，其中 `userId / title / sourceSystem / sourceId` 必填。
- `PATCH /api/v1/todos/:id/done`：本人标记完成（不存在或已完成返回 404 `A04007`）。
- 写操作均记审计（`TODO_UPSERT` / `TODO_DONE`）。
- 对接方当前复用门户管理员会话（Cookie + CSRF）调用；独立的 service account / API token 认证属 Phase 2。

## 12. 邮件模块（企业邮箱，独立 Mail 领域）

> 当前 Wave 1 基座尚未提供 Mail 运行时接口；生产前端不展示邮件入口。下述 Provider/表结构是后续
> 实施边界，不代表当前版本已具备企业邮箱能力。

定位：待办中心的一个独立 Tab，与 Todo 平级解耦——**邮件默认不进入待办**，仅用户手动"转为待办"时建立引用关联（`todos.source_system='mail'` + `source_id=mail_messages.id`，复用既有幂等机制，不改 Todo 主表、不建桥接表）。

**Provider 抽象**：业务层只面向 `mail.Provider` 接口（TestConnection / FetchInbox / FetchBody / SetFlags / Capabilities），不感知厂商。当前统一为 Generic IMAP 实现，阿里/腾讯等厂商差异收敛在 `Profile` 默认主机配置（`imap.qiye.aliyun.com:993` 等）；未来 Microsoft Graph / Exchange API 以新 Provider 实现接入，业务与前端零改动。API 不暴露 Provider 细节（无 `/api/aliyun/...` 式端点）。

**表结构**（`0003_mail.sql`）：`mail_accounts`（凭证 AES-256-GCM 密文，密钥来自 `MAIL_CREDENTIAL_KEY`，开发缺省由 `SESSION_SECRET` 派生；密钥与密文不同库）、`mail_messages`（按 `(account_id, folder, uid)` 幂等；正文按需拉取后缓存，附件只留 `has_attachment` 元数据）、`mail_sync_state`（UIDVALIDITY / last_uid 游标）。

**同步**：手动"同步"按钮 + 服务端定时补偿（`MAIL_SYNC_INTERVAL_MINUTES`，默认 10 分钟，0 禁用）。只同步最近 50 封元数据；正文 PEEK 按需拉取（不触发服务端已读）。IMAP IDLE 实时推送、断线指数退避重连、多实例 Lease（DB lease，不为此引入 Redis）属 Phase 2；SMTP 回复/转发、附件下载属 Phase 3。

**API**：`GET /mail/providers`（Profile + capabilities）、`GET/POST/DELETE /mail/accounts`（绑定前实测连接，通过才落库）、`POST /mail/accounts/:id/test|sync`、`GET /mail/messages`（unread/starred/keyword/分页）、`GET /mail/messages/:id`（打开即已读）、`POST /mail/messages/:id/read|star|todo`。

**安全**：邮件 HTML 前端 DOMPurify 消毒（禁 script/iframe/object/embed/form 等），远程图片默认拦截（防追踪像素），用户手动"显示图片"；日志不输出凭证/正文。
