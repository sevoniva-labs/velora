# Velora 产品级完成矩阵

日期：2026-08-24

范围：门户、管理后台、管理 API、应用接入、生产发布

约束：`main` 分支；保留既有门户和 ProComponents 视觉体系；不修改 Casdoor；不启用 Velora 自建 OIDC Provider

## 判定规则

- **完成**：存在可执行实现、自动化验证和生产证据。
- **代码完成**：实现与自动化已通过，等待本次生产发布或人工页面确认。
- **外部待办**：必须依赖新增基础设施、第三方测评或人工人机验证，仓库只能提供门禁、适配和证据模板。

## 产品与流程

| 要求 | 实现证据 | 自动化证据 | 当前结论 |
| --- | --- | --- | --- |
| Velora 是唯一日常入口，Casdoor 不暴露普通管理界面 | `docs/application-integration-standard-v1.md`、Portal 登录与 OIDC gateway | 登录、退出、Spectra 联邦退出验收 | 完成 |
| 无应用权限不是错误 | 门户空态与默认拒绝授权 | `product-smoke.spec.ts`：无应用空态 | 完成 |
| 后台菜单按管理员任务分组，菜单与直达地址权限一致 | `layout/menu.tsx`、`AdminLayout.tsx`、路由权限守卫 | `critical-flows.spec.ts`：权限菜单与直达页面 | 完成 |
| 组织、部门、岗位、用户组、用户、平台角色均在 Velora 管理 | `pages/admin/Organization.tsx`、`UserGroups.tsx`、`Users.tsx`、`Roles.tsx` | Go service/API 测试、Web 单元测试、全量构建 | 完成 |
| 用户生命周期包含停用/恢复、解锁、审批重置密码、MFA 状态、会话处置 | `pages/admin/UserDetail.tsx`、`Sessions.tsx`、用户管理 API | 后端用户/审批/会话测试；MFA 浏览器用例 | 完成 |
| 应用详情集中管理，不再把接入拆成多个顶级技术菜单 | `pages/admin/ApplicationManagement.tsx` 8 个业务页签 | E2E：8 页签逐页加载且无失败态 | 完成 |
| 应用角色与谁能访问应用分开 | `ApplicationAccess.tsx` 的角色视图与使用范围视图 | E2E：应用角色页签、四类使用范围 | 完成 |
| 使用范围支持部门、用户组、平台角色、指定人员及最终有效权限 | 授权计算 API 与 `ApplicationAccess.tsx` | E2E：四类对象预览、保存和影响人数 | 完成 |
| 审批通过后必须继续执行，不把“批准”误当成“已生效” | `Approvals.tsx`、业务深链和 approvalId 执行 | E2E：审批决定 → 业务深链 → 执行 | 完成 |
| 账号下发支持失败可诊断、重试、目标修改和一次性密钥 | `ApplicationProvisioning.tsx`、可靠消息与签名协议 | E2E：失败 → 重试 → 修改 → 一次性密钥 | 完成 |
| 应用停用/恢复状态即时一致 | `ApplicationInformation.tsx` 同时刷新列表和详情缓存 | E2E：停用、恢复及详情即时更新 | 完成 |
| MFA 默认可选，用户可自行开启；风险时再展示 Turnstile | `UserCenter.tsx`、登录风险处理、服务端 Turnstile 校验 | E2E：MFA 注册恢复码；登录 MFA + Turnstile 串联 | 完成 |
| 退出清理会话并回到 Velora，不输出 Casdoor JSON | Portal logout 编排 | E2E + 既有生产 Carson/Spectra 验收 | 完成 |

## 界面、文案与可用性

| 要求 | 实现/验证证据 | 当前结论 |
| --- | --- | --- |
| 保留原顶栏、蓝色主题、分组菜单和 ProComponents 页面骨架 | `AdminLayout.tsx`、`theme/tokens.ts`、`styles/admin-pro.css` | 完成 |
| 工作台和应用概览卡片不重叠 | 独立 CSS Grid、卡片间距；桌面几何重叠断言 | 完成 |
| 详情使用标准 PageContainer Tabs，不使用自绘页签 | `ApplicationManagement.tsx` | 完成 |
| 分页器位于表格底部且页面高度稳定 | `styles/admin-pro.css` 主/次表格布局；审批页几何断言 | 完成 |
| 窄屏和 200% 缩放不横向溢出 | 响应式样式、移动项目和缩放 E2E | 完成 |
| 不出现“下一项”等不明确字段；必要文案使用业务中文 | `labels.ts`、应用列表/概览与全仓文案走查 | 完成 |
| 表单可见标签能关联复杂人员选择器 | `AdminUserSelect.tsx` 透传 `id` | 单元测试 + 四类授权 E2E 通过 |
| 登录与后台无 serious/critical axe 问题 | `product-smoke.spec.ts` axe 基线 | 完成；不等同于完整 WCAG 认证 |

## 工程、生产与回滚

| 要求 | 证据 | 当前结论 |
| --- | --- | --- |
| 国内依赖源 | `goproxy.cn`、`registry.npmmirror.com`、DaoCloud 镜像策略及 CI 检查 | 完成 |
| E2E 纳入统一门禁 | `server/Makefile` 的 `ci-web-e2e`，`verify` 强制依赖 | 完成 |
| 可追溯版本 | `BUILD-INFO`、`SHA256SUMS`、API version、OCI revision/version | 完成 |
| 安全与供应链门禁 | gosec、govulncheck、gitleaks、staticcheck、golangci-lint、SBOM/签名策略 | 完成 |
| 生产依赖、健康、备份、归档和证书巡检 | `docs/product-grade-production-delivery-report-2026-08-24.md` | 完成；本次增量发布后已复验 |
| 可恢复发布 | 发布前镜像标签、配置副本、PostgreSQL pgdump；应用回滚不破坏新增表 | 完成；回滚点 `20260824T155255Z-before-5d23528` |

## 不得伪造为已完成的外部项目

| 项目 | 当前准备 | 取得“金融级”结论所需证据 |
| --- | --- | --- |
| 多可用区高可用 | 健康检查、无状态服务和部署模板 | 第二节点/托管 HA、真实故障切换记录 |
| 异地灾备 | 加密备份、远端副本、恢复脚本 | 独立区域恢复演练与实测 RPO/RTO |
| 国密 KMS/HSM | SM 算法接口、适配器门禁、轮换流程 | 经批准设备/云服务、密钥全生命周期和故障演练 |
| WORM/SIEM | 签名归档、可靠出站队列、证据模板 | 受监管存储与 SIEM 实际接收、查询和告警 |
| 合规认证 | 自动化安全扫描与加固基线 | 第三方渗透、等保/内控或行业测评报告 |
| 生产管理员页面确认 | 仓库 E2E 已覆盖后台页面与写流程 | 管理员完成一次真实 Turnstile 人机点击后的只读页面验收 |

## 本轮发布退出条件

1. `make verify` 全绿，E2E 以逻辑场景数和项目展开数分别报告。
2. 代码以 Conventional Commits 提交并推送 `main`，不包含用户未跟踪截图资产。
3. 使用国内源构建，生产 Server/Worker/Web 的 revision 一致。
4. 发布前生成可恢复回滚点；发布后健康、就绪、OIDC discovery、Demo 和 Spectra 均通过。
5. 人工 Turnstile 仍未确认时，必须明确标记为外部人工验收，不得把自动化测试冒充生产人机验证。
