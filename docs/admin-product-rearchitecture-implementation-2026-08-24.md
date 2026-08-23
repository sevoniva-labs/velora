# Velora 管理后台重构实施与回滚说明

状态：代码、迁移和生产制品已部署；等待登录后的浏览器验收。

## 交付范围

- 管理后台使用 `ProLayout`、`PageContainer`、`ProTable`、`ProDescriptions`、`EditableProTable` 和 `ProForm` 构建统一骨架。
- 顶层信息架构收敛为工作台、应用中心、组织与人员、权限治理、安全与审计、平台设置。
- 应用资料、登录、应用角色、访问范围、账号同步、验证发布和变更记录进入同一应用详情。
- 访问范围支持全体成员、部门及下级部门、用户组、平台角色、人员和排除规则；默认拒绝，排除优先，允许角色取并集。
- 有效权限统一驱动门户访问和账号同步，并保存权限来源。
- 高风险或至少 50 个同步任务的访问变更自动走申请、审批、执行；界面不接收原始审批编号。
- 已交付审批、临时授权、访问复核、配置发布与回滚、服务账号、在线会话和操作审计。
- Casdoor 不进入日常管理导航；未修改 Casdoor，未建设 Velora OIDC Provider。

## 数据变更

PostgreSQL 迁移 `00028` 新增应用访问规则、规则角色和有效权限来源；`00029` 新增应用负责人和所属部门引用。迁移均为 additive，不删除旧策略或旧 entitlement。生产当前仅对 PostgreSQL 给出验收结论。

升级前必须备份 Velora 与 Casdoor 数据库，并记录旧 Server、Worker、Web 镜像 ID。迁移后旧表保留，可供旧版本读取；不得在事故窗口删除新表或回写历史迁移。

## 测试门禁

```text
web: pnpm lint
web: pnpm test --run
web: pnpm build
server: GOPROXY=https://goproxy.cn go test ./...
server: make proto-check
server: make contract
```

生产验收必须覆盖：登录、无权限合法空态、管理员菜单边界、应用详情七个页签、部门/组/角色/人员授权、排除优先、审批执行、账号同步、应用停用恢复、审计和服务健康。

## 发布顺序

1. 冻结管理写入并执行数据库备份。
2. 本机构建 `linux/amd64` Server、Worker、Migrate 和 Web 静态制品，记录 SHA-256。
3. 上传新制品并给旧镜像加回滚标签。
4. 运行一次 Migrate；失败立即停止，不替换运行容器。
5. 依次替换 Server、Worker、Web，等待健康检查通过。
6. 运行公网健康、登录和管理 API 验收，确认后解除写入冻结。

## 回滚

1. 停止新的应用授权、账号下发和配置发布。
2. 将 Server、Worker、Web 恢复为发布前镜像并等待健康。
3. 不回滚 additive migration，不删除 `application_access_*` 或负责人字段。
4. 验证登录、应用列表、旧访问读取、审计和 Worker；对失败窗口内的可靠消息执行对账。
5. 若一次性 Secret 已签发或暴露，只能轮换，不能恢复旧值。

回滚触发条件：迁移失败、服务未就绪、登录链路失败、管理员主要路由 5xx、授权集合与发布前基线不符，或账号同步持续失败。

## 生产实施记录

实施时间：2026-08-24（Asia/Shanghai）。目标：`ubuntu@175.27.250.53`。

- 生产备份服务执行成功，退出码为 0。
- Server、Worker、Migrate 使用本机构建的 `linux/amd64` 制品；Web 在本机构建后上传。依赖使用 `goproxy.cn`，服务器未执行 Go 或前端编译。
- Server/Worker 制品版本：`4290280`；Web 最终版本：`c62b809`。
- PostgreSQL additive migration 成功；`application_access_grants`、`application_access_grant_roles`、`user_application_entitlement_sources` 及应用负责人字段均已存在。
- 旧策略迁移后有 3 条访问规则、1 条权限来源，现有 2 个应用保留。
- Server、Worker、Web、PostgreSQL、Redis、Casdoor、Edge 与 Demo 容器健康。
- `home` 健康、API health、API readiness、OIDC Discovery 与 Demo health 均返回 HTTP 200；readiness 的 database、cache、messaging、search、storage 均为 `UP`。
- 新管理 API 在未认证状态统一返回 401，无 5xx；Server 发布后未发现 error、panic 或 fatal 日志。
- 发布前镜像保留为 `rollback-pre-4290280` 与 `rollback-pre-c62b809` 标签；制品和校验文件位于 `/opt/velora/prod/releases/4290280`、`/opt/velora/prod/releases/c62b809`。

最终自动门禁：Web lint、54 项测试和生产构建通过；Server `go test ./...`、Proto、OpenAPI 与 122 条 HTTP 契约门禁通过。

浏览器已验证未认证访问 `/admin` 正确回到 Velora 登录页，页面不暴露 Casdoor。登录后的菜单、应用详情和治理操作仍需由人工完成一次 Turnstile 后继续验收；不得把该项记录为已通过。
