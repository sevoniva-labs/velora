# Phase 0–4 实施与验收记录

状态：历史本地实施记录。下述“待外部验收”反映 2026-08-21 当时状态；该项已于 2026-08-24 在生产完成，当前结论与版本以 [应用接入生产验收记录](./application-onboarding-production-evidence-2026-08-24.md) 为准。

## 已交付

- Phase 0：移除未落库的 Client ID/应用名假字段，正式表单开放直链并允许创建待验收的 OIDC 草稿；普通用户界面统一使用“统一身份中心”；管理入口和菜单按细粒度权限集合渲染；旧 SAML/CAS/ForwardAuth 在向导中明确禁用。
- Phase 1：新增 `00023_portal_identity_boundary` 与 `00024_portal_identity_scopes` additive migration；身份绑定、验证记录、生命周期、发布时间、操作者、`config_version` 乐观锁和 OIDC Scopes；`iam.integration.*`、`iam.console.open`、`portal.application.publish` 权限；控制台 URL 仅通过受权 API 返回；后端拒绝新建未验收的 SAML/CAS/ForwardAuth 绑定。门户应用、分类、标签、访问策略的管理写操作，以及身份验证、提交发布、发布、禁用均使用 DB-backed 幂等记录；支持 `Idempotency-Key`，缺省按确定性请求指纹保护浏览器重试，审计仍在同一事务内完成。
- 权限模型：新增目标权限 `audit.read`，同时保留 `system.audit.read` 兼容旧角色和令牌；管理后台入口、菜单和身份向导按钮按权限集合渲染，身份管理员可读取应用清单但不能越权修改门户或打开未授权控制台；集成令牌增加 `system.api_token.manage` 专用权限。
- Phase 2：新增“身份与单点登录”五步向导，支持 URL/OIDC 分支、绑定保存、真实 Discovery 验证、策略跳转、验证失败恢复和发布门禁；向导草稿只保存非敏感字段到浏览器本地存储并可恢复；不收集 Client Secret；管理页展示连接状态、Issuer 和待处理应用数量。
- Phase 3：保留并修正本地 OIDC PKCE smoke；已实测本地 Casdoor Discovery、Velora begin、`state`/`nonce`/S256 PKCE 参数、未授权管理接口拒绝和容器健康；OIDC `amr/acr` 中的 MFA 证据会进入 Velora 会话认证等级。完整登录、MFA、撤权、错误回调和登出必须使用真实测试账号执行。
- Phase 4：新增默认关闭的最小权限 Casdoor 应用自动化 Provider 抽象和只读检查命令；只允许应用客户端管理，强制真实 `approval_id` 执行票据和 MFA，支持 OIDC Scopes、Redirect URI 幂等更新、禁用及同票据失败重试，不管理用户/密码/角色/MFA；新建客户端的 Secret 仅返回给同时拥有身份管理和受控控制台权限的管理员一次，随后清理内存，不进入数据库、日志、审计或浏览器持久化存储；身份绑定重试只缓存脱敏响应。
- 安全收口：门户首页/启动 URL 仅接受无用户信息、无查询串、无片段的 HTTPS 地址；`audit.read` 与历史 `system.audit.read` 在授权层等价，避免新权限分配后误拒绝旧审计接口。
- 生产认证收口：生产配置明确拒绝 `VELORA_CASDOOR_PASSWORD_LOGIN_ENABLED=true`，禁止 Velora 代转 Casdoor 密码/ROPC；生产只允许标准 OIDC Authorization Code + PKCE。Turnstile 仍作为本地和生产登录风控配置项。
- 身份映射边界：Casdoor 的用户、组织、角色和用户组仍是权威事实；Velora 只通过经过审批的 `external_identities` 绑定把 Casdoor subject 映射到本地用户，再按 Velora 本地角色/用户组计算门户权限，不根据未经批准的 OIDC claims 自动授予高权限。

## 已执行命令

```text
server: go test ./...       PASS
server: go vet ./...        PASS
web:    pnpm test            PASS (42 tests)
web:    pnpm lint            PASS
web:    pnpm build           PASS
proto:  make -C server proto-check PASS
proto:  make -C server proto-generate PASS
shell:  bash -n scripts/local-casdoor-oidc-smoke.sh scripts/check-production-config.sh PASS
config: scripts/check-production-config.sh PASS; helm lint server/deploy/helm/forge PASS
git:    git diff --check     PASS; 工作区仅保留本次变更
secret: gitleaks protect --staged PASS
idempotency: DB-backed portal write retry reservation/replay implemented; one-time Client Secret responses are intentionally excluded from persistence
commit: `ef3b854 fix(identity): make binding retries secret safe`、`ef31663 feat(portal): make admin catalog writes retry safe`、`8e5c8ba refactor(casdoor): isolate application provider boundary`；此前收口提交均已推送 `origin/codex/velora-forge-backend-replacement`
commit: 6e18f1a/c5a829e/4240340/2be8c4f/9c7d73d/0fc5074/a2e368d/9cd86d4/0980293：Scopes 持久化、MFA assurance、审批重试、自动化 UI/部署 Secret、退役 stub 清理、权限驱动后台入口和 OIDC 草稿入口；均已推送
```

本地 Compose 证据（2026-08-21）：

- `docker compose --env-file .env.local up -d --build server web`：server/web healthy；
- `velora-migrate` 一次性执行完成，最终迁移应为 `goose_db_version=24`（包含绑定 Scopes）；
- 当前镜像重建后执行 `/usr/local/bin/velora-migrate` 成功；PostgreSQL `velora_dev` 的 `goose_db_version=24`，`portal_application_identity_bindings`、`portal_application_verifications`、`idempotency_records` 均存在；
- `GET /api/v1/system/health` 返回 200，Turnstile enabled；
- `GET /api/v1/admin/identity/overview` 无会话返回 401；
- 已实现身份管理员概览的受权 Discovery 探测与 Issuer 校验，返回连接状态、Issuer 和待处理应用数量；管理 URL 仍不进入公开健康接口；实际登录后验收待外部测试账号；
- 本地 Casdoor `/healthz` 返回 200，`/.well-known/openid-configuration` 返回 200，Issuer 为本地 Casdoor，Discovery 提供授权、Token、JWKS 和 RP-initiated logout 端点；
- Velora OIDC begin 返回 200，授权地址包含 `response_type=code`、`state`、`nonce`、`code_challenge_method=S256`、`code_challenge` 和配置的 redirect URI。
- 最终镜像已由当前代码重建；server、web、Postgres、Casdoor 容器 healthy；迁移命令再次执行成功且版本为 24。
- 生产 Compose 静态检查 `scripts/check-production-config.sh` PASS；Helm lint PASS；自动化 Token 仅通过 Secret 文件注入，默认 flag 仍为 false。
- 旧数据兼容性：从仅执行 00001–00022 的临时 PostgreSQL 数据库开始，插入既有 URL/OIDC 应用后执行 00023/00024；结果为 URL `PUBLISHED/ENABLED`、OIDC `IDENTITY_PENDING/DISABLED`，绑定/验证表与 `scopes_json` 均创建成功；临时数据库已销毁。
- 浏览器基础验收：本地登录页在桌面默认视口可读取账号/密码输入框、登录按钮和可访问名称；键盘填充账号/密码控件可正常获得焦点；移动视口 `390×844` 下页面保持 `body.scrollWidth=390`，无横向溢出且登录结构正常。后台完整菜单和发布流程仍需使用批准的 Casdoor 测试账号验收。

## 历史待验收项（现已完成）

本节保留用于说明当时没有使用 Mock 冒充外部证据。真实生产闭环结果、应用版本和门禁证据见 [2026-08-24 生产验收记录](./application-onboarding-production-evidence-2026-08-24.md)。

本机 Casdoor 只有内置管理员，没有可用于验收的批准测试账号、密码、MFA 设备及 Velora `external_identities` 映射。没有这些外部输入，不能合法完成登录/MFA/撤权、角色/用户组映射和下游 OIDC 全链路，也不能把结果标记为通过。执行方式：

```bash
CASDOOR_TEST_USERNAME='approved-test-user' \
CASDOOR_TEST_PASSWORD='provided-out-of-band' \
VELORA_OIDC_CLIENT_ID='provided-out-of-band' \
VELORA_OIDC_CLIENT_SECRET='provided-out-of-band' \
./scripts/local-casdoor-oidc-smoke.sh
```

凭据只通过 Secret Manager 或进程环境注入，不写仓库、日志或验收证据。

## 回滚

1. 设置 `VELORA_APPLICATION_ONBOARDING_V2=false`、`VELORA_CASDOOR_ADMIN_ENTRY_ENABLED=false`；
2. 回退 server/web 镜像到上一兼容提交；
3. 保留 00023/00024 additive 表和字段，不执行破坏性 Down migration；
4. 旧 URL 应用继续按原启动逻辑运行；
5. 导出回滚期间新增草稿、绑定和验证记录，恢复后按 `config_version` 重放；
6. 验证登录、应用列表、启动、权限拒绝和审计。

禁止删除新表、覆盖数据库或 force-push 回滚。
