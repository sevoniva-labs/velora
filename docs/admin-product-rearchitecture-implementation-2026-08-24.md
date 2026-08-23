# Velora 管理后台重构实施与回滚说明

状态：代码已完成并通过本地门禁；生产验收结果在部署后补录。

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
