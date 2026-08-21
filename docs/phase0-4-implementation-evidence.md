# Phase 0–4 实施与验收记录

状态：代码闭环已完成，真实 Casdoor 用户登录/MFA 验收仍需一组经批准的测试账号和身份映射；未使用 mock 冒充该证据。

## 已交付

- Phase 0：移除未落库的 Client ID/应用名假字段，正式表单只开放已闭环的直链；普通用户界面统一使用“统一身份中心”；管理入口和菜单按细粒度权限集合渲染；旧 SAML/CAS/ForwardAuth 在向导中明确禁用。
- Phase 1：新增 `00023_portal_identity_boundary` additive migration；身份绑定、验证记录、生命周期、发布时间、操作者和 `config_version` 乐观锁；`iam.integration.*`、`iam.console.open`、`portal.application.publish` 权限；控制台 URL 仅通过受权 API 返回；后端拒绝新建未验收的 SAML/CAS/ForwardAuth 绑定。
- 权限模型：新增目标权限 `audit.read`，同时保留 `system.audit.read` 兼容旧角色和令牌。
- Phase 2：新增“身份与单点登录”五步向导，支持 URL/OIDC 分支、绑定保存、真实 Discovery 验证、策略跳转、验证失败恢复和发布门禁；向导草稿只保存非敏感字段到浏览器本地存储并可恢复；不收集 Client Secret；管理页展示连接状态、Issuer 和待处理应用数量。
- Phase 3：保留并修正本地 OIDC PKCE smoke；已实测本地 Casdoor Discovery、Velora begin、`state`/`nonce`/S256 PKCE 参数、未授权管理接口拒绝和容器健康。完整登录、MFA、撤权、错误回调和登出必须使用真实测试账号执行。
- Phase 4：新增默认关闭的最小权限 Casdoor 应用自动化客户端和只读检查命令；只允许应用客户端管理，强制 `approval_id` maker-checker，不管理用户/密码/角色/MFA；新建客户端的 Secret 仅返回给同时拥有身份管理和受控控制台权限的管理员一次，随后清理内存，不进入数据库、日志、审计或浏览器持久化存储。

## 已执行命令

```text
server: go test ./...       PASS
server: go vet ./...        PASS
web:    pnpm test -- --run   PASS (38 tests)
web:    pnpm lint            PASS
web:    pnpm build           PASS
proto:  make -C server proto-check PASS
proto:  make -C server proto-generate PASS
shell:  bash -n smoke/init  PASS
git:    git diff --check     PASS; 工作区仅保留本次变更
secret: gitleaks protect --staged PASS
```

本地 Compose 证据（2026-08-21）：

- `docker compose --env-file .env.local up -d --build server web`：server/web healthy；
- `velora-migrate` 一次性执行完成，`goose_db_version=23`；
- `portal_application_identity_bindings` 与 `portal_application_verifications` 存在；
- `GET /api/v1/system/health` 返回 200，Turnstile enabled；
- `GET /api/v1/admin/identity/overview` 无会话返回 401；
- 已认证身份管理员概览真实探测 Discovery 并校验 Issuer，返回连接状态、Issuer 和待处理应用数量；管理 URL 仍不进入公开健康接口；
- 本地 Casdoor `/healthz` 返回 200，`/.well-known/openid-configuration` 返回 200，Issuer 为本地 Casdoor，Discovery 提供授权、Token、JWKS 和 RP-initiated logout 端点；
- Velora OIDC begin 返回 200，授权地址包含 `response_type=code`、`state`、`nonce`、`code_challenge_method=S256`、`code_challenge` 和配置的 redirect URI。

## 未伪造的外部验收项

本机 Casdoor 只有内置管理员，没有可用于验收的批准测试账号、密码、MFA 设备及 Velora `external_identities` 映射。没有这些外部输入，不能合法完成登录/MFA/撤权全链路，也不能把结果标记为通过。执行方式：

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
3. 保留 00023 additive 表和字段，不执行破坏性 Down migration；
4. 旧 URL 应用继续按原启动逻辑运行；
5. 导出回滚期间新增草稿、绑定和验证记录，恢复后按 `config_version` 重放；
6. 验证登录、应用列表、启动、权限拒绝和审计。

禁止删除新表、覆盖数据库或 force-push 回滚。
