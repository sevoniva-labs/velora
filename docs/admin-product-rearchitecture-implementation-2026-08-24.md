# Velora 管理后台重构实施与回滚说明

状态：Phase 0–4 主体、迁移、自动门禁和生产部署已完成；MFA 与真实登录已验收，管理员全菜单目视验收和 Turnstile 托管模式切换尚未完成。

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
- 当前 Server 版本：`08b0e4b`；当前 Web 版本：`bd37968`。除既有应用、组织、权限、审批和审计能力外，当前版本不再公开 Casdoor 账号地址；Casdoor 仅在 Velora 服务端完成主凭据校验。用户可在 Velora 个人中心启用或停用 TOTP、保存一次性恢复码，并在 Velora 登录页使用验证码或恢复码。高风险管理操作遇到 `STEP_UP_REQUIRED` 时会打开 ProComponents 身份确认窗口，完成密码与 MFA 校验后由用户重试原操作。
- PostgreSQL additive migration 成功，当前 Goose 版本为 `31`；`application_access_grants`、`application_access_grant_roles`、`user_application_entitlement_sources`、应用负责人字段、平台角色生命周期字段和 `user_role_exclusions` 均已存在。
- 旧策略迁移后有 3 条访问规则、1 条权限来源，现有 2 个应用保留。
- Server、Worker、Web、PostgreSQL、Redis、Casdoor、Edge 与 Demo 容器健康。
- `home` 健康、API health、API readiness、OIDC Discovery 与 Demo health 均返回 HTTP 200；readiness 的 database、cache、messaging、search、storage 均为 `UP`。
- 通过一次性生产验收身份完成认证管理 API 验收：6 个平台角色、3 个用户、2 个应用均可读取；`carson` 的有效应用权限可解释；Spectra 账号下发重试保持 `HEALTHY`；旧单用户 entitlement 和旧访问策略写接口均返回 400，生产应用物理删除返回 400，确认旧写入面已退役。最终版本进一步验证 `/api/v1/admin/users?page=1&page_size=1&keyword=carson&status=ACTIVE` 返回唯一的 `carson`，并验证 `/api/v1/admin/portal/applications?page=1&page_size=1&keyword=spectra&status=ENABLED` 返回唯一的 `Spectra`；两者均返回 `total=1`、`page=1`、`page_size=1`。当前版本完成 22 个生产管理接口矩阵验证；审批和配置变更明确要求交互式用户会话，机器令牌得到稳定 403，临时交互式会话验证两接口均返回 200。配置变更列表此前把“需要交互式会话”错误映射为 500，已修正为 403 并增加回归测试。所有临时令牌和会话随后删除，数据库计数为 0，管理员强制改密标志恢复。
- Server 发布后未发现 error、panic 或 fatal 日志。Worker 在未配置 WORM 归档时明确记录 `WARN` 并禁用清理，不会误报故障，也不会在没有不可变归档时删除审计数据。
- 当前回滚标签为 Server `rollback-pre-08b0e4b`、Web `rollback-pre-bd37968`；更早标签继续保留。Server 制品位于 `/opt/velora/prod/releases/08b0e4b/server`，主二进制 SHA-256 为 `31f6afc55c12155c667585503276d0a986255a02be328384480fe68b83c4f771`；Web 制品位于 `/opt/velora/prod/releases/bd37968/web`，入口文件 SHA-256 为 `ab387bc5720f3aa19a158dfaf694cc8833bac7e866c0a372e2dcc40e5de7cec1`，生产入口加载 `assets/index-CiwBrhhc.js`。生产镜像 ID 分别为 Server `fbe19ce6d6d4`、Web `1cfab3a454ec`。

最终自动门禁已在当前版本重新执行：Web lint、13 个测试文件共 59 项测试和生产构建通过；Server `go test ./...` 通过。新增回归用例覆盖不可见 Turnstile 配置和 `STEP_UP_REQUIRED` 前端事件；既有授权元测试继续覆盖 Platform、Approval、Portal 全部 gRPC 操作。

浏览器已验证未认证访问 `/admin` 正确回到 Velora 登录页，页面和公共健康接口均不暴露 Casdoor。生产仍采用风险触发：正常首次凭据提交不加载 Turnstile；凭据失败后才为 IP 和标准化账号写入 15 分钟挑战状态，成功登录清理状态，原有 IP/账号限流、账号锁定和审计继续生效。真实 `carson` 凭据已通过 Velora 页面登录并进入 `/home`，未再出现验证不可用、转圈或网络异常。

Cloudflare 控制台确认 Site Key 的唯一 hostname 是 `home.sevoniva.com`、action 是 `login`，服务器到 Siteverify 网络正常。当前 Widget 为 Invisible；生产实测风险命中后不会提供可交互挑战，按钮会等待令牌，因此不能作为最终产品配置。最终方案是把同一 Widget 切换为 Managed，前端改用 `appearance=interaction-only`：正常用户不显示额外框，只有 Cloudflare 要求交互时显示官方挑战。完成该外部安全配置和二次真实登录前，不得把 Turnstile 验收记为通过。

## 最终生产核验快照

核验时间：2026-08-24（Asia/Shanghai）。

- 公网 `/api/v1/system/health` 与 `/api/v1/system/ready` 均返回成功，database、cache、messaging、search、storage 全部为 `UP`。
- Server、Web、Worker、OIDC Demo、Redis、Casdoor、Edge、PostgreSQL 全部健康；当前生产 Server 为 `08b0e4b`，Web 为 `bd37968`，Web 入口为 `assets/index-CiwBrhhc.js`；公共健康不再返回 Casdoor 地址。
- 数据库迁移版本为 31；平台角色 6 个；最终分页验收后临时令牌 0 个；`admin.must_change_password=true` 已恢复。
- `user_role_exclusions` 当前为 0 条是正常生产数据状态；迁移、外键和访问复核撤权代码路径已通过自动测试。
- WORM 归档适配器属于已明确预留能力；在正式配置不可变归档前，审计清理保持关闭。这不是数据保留门禁失败，也不得手工开启清理。
