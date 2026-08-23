# Velora 管理后台重构实施与回滚说明

状态：Phase 0–4 代码、迁移、自动门禁、生产部署和认证管理 API 验收已完成；仅剩登录后浏览器目视验收。

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

PostgreSQL 迁移 `00028` 新增应用访问规则、规则角色和有效权限来源；`00029` 新增应用负责人和所属部门引用；`00030` 补齐平台角色说明、启停状态和唯一约束；`00031` 新增用户角色排除记录，使访问复核能够真正撤销用户组继承角色。迁移均为 additive，不删除旧策略或旧 entitlement。生产当前仅对 PostgreSQL 给出验收结论。

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

## 门户展示配置

门户名称、欢迎语、页脚、公告和界面缩放属于 Web 发布配置，不伪装成运行时管理 API，也不保存在浏览器本地存储。构建 Web 制品前可设置：

```text
VITE_PORTAL_NAME
VITE_PORTAL_WELCOME
VITE_PORTAL_FOOTER
VITE_PORTAL_ANNOUNCEMENT
VITE_UI_SCALE
```

未设置时使用产品默认值。任何变更都必须生成新的 Web 制品、记录校验值并按正常发布/回滚流程上线。

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
- Server 与 Migrate 基础制品版本：`ab0f83c`；Server 热修复版本：`ea8478d`；Worker 最终版本：`7505112`；Web 最终版本：`18600b1`。
- PostgreSQL additive migration 成功，当前 Goose 版本为 `31`；`application_access_grants`、`application_access_grant_roles`、`user_application_entitlement_sources`、应用负责人字段、平台角色生命周期字段和 `user_role_exclusions` 均已存在。
- 旧策略迁移后有 3 条访问规则、1 条权限来源，现有 2 个应用保留。
- Server、Worker、Web、PostgreSQL、Redis、Casdoor、Edge 与 Demo 容器健康。
- `home` 健康、API health、API readiness、OIDC Discovery 与 Demo health 均返回 HTTP 200；readiness 的 database、cache、messaging、search、storage 均为 `UP`。
- 通过一次性生产验收令牌完成认证管理 API 验收：6 个平台角色、3 个用户、2 个应用均可读取；`carson` 的有效应用权限可解释；Spectra 账号下发重试保持 `HEALTHY`；旧单用户 entitlement 写接口返回 400，确认已退役。验收令牌和会话随后删除，数据库计数均为 0。
- Server 发布后未发现 error、panic 或 fatal 日志。Worker 在未配置 WORM 归档时明确记录 `WARN` 并禁用清理，不会误报故障，也不会在没有不可变归档时删除审计数据。
- 发布前镜像保留为 `rollback-pre-ab0f83c`、`rollback-pre-ea8478d`、`rollback-pre-7505112` 和 `rollback-pre-18600b1`；制品位于 `/opt/velora/prod/releases/ab0f83c`、`/opt/velora/prod/releases/7505112` 与 `/opt/velora/prod/releases/18600b1`。最终 Worker SHA-256 为 `0cdd6a13ba15249c64c241be0ef001cc8166072feb828cb0ce9ff1e6e60c22d1`，最终 Web 压缩制品 SHA-256 为 `5142ed4af50d3ef47c4367377f9a662b741a5e33ade662b5891e2bf88a187b0a`。

最终自动门禁：Web lint、11 个测试文件共 54 项测试和生产构建通过；Server `go test ./...`、Proto、OpenAPI 与 127 条 HTTP 契约门禁通过，其中 76 条写操作受 CSRF 保护。

浏览器已验证未认证访问 `/admin` 正确回到 Velora 登录页，页面不暴露 Casdoor。生产曾因前端自设的 12 秒计时把仍在运行的 Turnstile 挑战误判为失败；`18600b1` 已移除该错误计时，生产等待 15 秒后仍正常显示 Cloudflare 验证控件且无控制台错误。自动化不得代替用户完成验证码；登录后的菜单、应用详情和治理操作仍需人工完成一次验证码后继续目视验收，不得把该项记录为已通过。

## 最终生产核验快照

核验时间：2026-08-24（Asia/Shanghai）。

- 公网 `/api/v1/system/health` 与 `/api/v1/system/ready` 均返回成功，database、cache、messaging、search、storage 全部为 `UP`。
- Server、Web、Worker、OIDC Demo、Redis、Casdoor、Edge、PostgreSQL 全部健康；最新 Worker 日志仅包含预期的 WORM 未配置警告。
- 数据库迁移版本为 31；平台角色 6 个；生产验收临时令牌 0 个、临时会话 0 个；`admin.must_change_password=true` 已恢复。
- `user_role_exclusions` 当前为 0 条是正常生产数据状态；迁移、外键和访问复核撤权代码路径已通过自动测试。
- WORM 归档适配器属于已明确预留能力；在正式配置不可变归档前，审计清理保持关闭。这不是数据保留门禁失败，也不得手工开启清理。
