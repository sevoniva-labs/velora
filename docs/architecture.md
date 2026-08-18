# Velora 架构与设计决策

本文档记录 Velora 第一阶段的关键架构决策与边界，供后续开发者快速理解。

## 1. 身份边界（最重要）

> **Casdoor manages identity. Velora manages the workspace.**

- Casdoor：身份 / IAM / SSO（OIDC Provider）。
- Velora：门户 / 工作台 / 应用枢纽。
- Velora **永不**直连 Casdoor 数据库；只通过 OIDC（登录）、Casdoor API（后续扩展）消费身份。
- 数据库隔离：同一 PostgreSQL Server，独立 database `casdoor` / `velora`。

## 2. 登录流（OIDC Authorization Code + PKCE）

```text
访问 Velora → GET /api/v1/me 401 → /login
→ GET /api/v1/auth/oidc/login?redirect=/home
   （生成 HMAC 签名 state：{redirect, code_verifier, nonce, exp}）
→ 302 Casdoor authorize?code_challenge=...
→ Casdoor 认证 → 302 回调 /api/v1/auth/oidc/callback?code&state
→ 校验 state（签名 + 过期）→ code+verifier 换 token
→ 校验 id_token（签名 + nonce）→ UserInfo
→ 建立 HttpOnly Session Cookie（HMAC 签名，无状态）
→ 302 回站内 redirect
```

安全要点：

- state 无状态自包含（HMAC + 过期），不依赖服务端存储；
- nonce 校验防重放；PKCE verifier 随 state 传递；
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
- 管理员：持有 `VELORA_ADMIN_ROLE`（默认 `velora_admin`，来自 Casdoor 角色）可管理应用/分类/标签/策略/审计。

## 5. Launch 安全

- 不接受客户端 URL（`POST /applications/:id/launch`，服务端读库）。
- `LaunchProvider` 扩展点：`URL`（受信配置地址，校验 http/https + host）、`OIDC`（Casdoor 为该应用签发跳转）。
- `SAML` / `CAS` / `FORWARD_AUTH` 仅保留模型与扩展点，未实现时明确报错（不做假实现）。

## 6. 模块边界

```text
cmd/velora        serve / migrate / seed
internal/auth      OIDC + Session（审计通过回调注入，避免包循环）
internal/application  应用域（模型 / 可见性 / Launch / 健康检查）
internal/category|tag|favorite|visit   独立领域
internal/permission   管理员中间件（公共）
internal/audit        审计服务（不依赖 auth）
internal/portal       门户设置与统计
internal/platform     config / db / errs / response / httpserver
```

## 7. 前端复用 Spectra 的边界

- 复用：`theme/tokens.ts`（Design Token）、`index.css` 样式体系（前缀改造为 `velora-`）、`api/client.ts` 请求封装（适配 Velora 统一返回）、`RequireAuth` / `useMe`、顶栏布局结构、工程配置（Vite / tsconfig / oxlint）。
- 重写：业务页面（门户/管理后台）、领域 API、菜单内容、标签映射。
- 禁止：复制 Spectra DevOps 业务逻辑；运行时依赖 Spectra 本地路径；修改 Spectra 目录。

## 8. 数据库迁移

- `server/migrations/*.sql`，按文件名顺序执行，`schema_migrations` 记录，幂等。
- 通过 `//go:embed` 嵌入二进制（`migrations` 包），`velora serve` 启动时自动迁移。

## 9. 已知权衡（Phase 1 明示）

- **无状态会话**：Session Cookie 为 HMAC 签名、无服务端存储。角色变更（如管理员回收）最长延迟一个 `SESSION_TTL_HOURS`（默认 168h，可按需调小）；登出只清除本地 Cookie，被盗 Cookie 无法主动作废（生产可引入服务端会话/黑名单，属 Phase 2）。
- **PKCE verifier 随 state 传递**：stateless state 内包含 code_verifier，会出现在回调 URL 中（浏览器历史/Referrer）。OIDC 库对 verifier 的时效敏感且 code 一次性，风险有限；如需更强隔离，可改为服务端短时存储 state（Phase 2 选项）。
- **列表过滤在内存完成**：`ListPublic` 先查库再按访问策略过滤+内存分页。应用量小时简单可靠；量大后应将策略过滤下沉到 SQL（Phase 2 优化）。

## 10. 中国大陆开发环境

- Go：`GOPROXY=https://goproxy.cn,direct`（`scripts/bootstrap-cn.sh` 提供，不修改全局配置）。
- pnpm：`web/.npmrc` 项目级 `registry=https://registry.npmmirror.com`。
- Docker：`DOCKER_REGISTRY` ARG 支持 `docker.m.daocloud.io` ↔ `docker.io` 切换；镜像锁定稳定版本。
