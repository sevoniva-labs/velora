# Velora 产品级生产交付报告

日期：2026-08-24  
目标环境：`ubuntu@175.27.250.53`  
生产版本：`0.2.0+897c8218c5fa`  
生产提交：`897c8218c5faaf7dc408ba54d831551c82638a57`

## 1. 交付结论

Velora 已达到“受控单节点 V1”的产品交付标准：门户、登录、应用目录、组织、用户、角色、
应用访问范围、审批、权限复核、审计、MFA 自助开通、OIDC 接入环境和账号下发均有可执行
产品流程；服务已部署到生产，核心依赖、OIDC、Demo、Spectra、备份、审计归档、证书续期和
健康巡检均处于正常状态。

本结论不等同于金融级基础设施认证。当前只有一台应用服务器，尚未取得多可用区高可用、
异地恢复演练、外部 WORM/SIEM、真实国密 KMS/HSM、渗透测试和等级保护测评的第三方证据。
这些事项必须在对应资源落实后验收，不能以代码适配或文档代替真实证明。

## 2. 产品与身份边界

- Velora 是用户入口和管理员日常工作台，承载组织、人员、角色、应用目录、应用访问范围、
  审批、权限复核、审计和应用接入流程。
- Casdoor 只作为外部身份提供方和 OIDC 协议端，不向普通用户暴露管理界面，不修改其源码，
  不把业务授权模型放回 Casdoor。
- Velora 不提供自建 OIDC Provider；接入应用使用 `auth.sevoniva.com` 的 OIDC 能力，门户登录页
  继续承载账号密码、风险触发 Turnstile 和可选 MFA。
- 应用访问由 Velora 统一计算，支持全员、部门（含下级）、用户、角色及排除规则；应用内部
  角色与“谁可以进入应用”分开配置。

## 3. 本轮产品整改

### 3.1 后台信息架构与视觉

- 保留既有门户/后台顶栏、分组菜单、蓝色主题和 ProComponents 组件体系，没有替换视觉基线。
- 应用管理拆分“应用角色”和“使用范围”，避免把协议配置、应用角色和人员授权混在同一向导。
- 应用详情使用标准 ProComponents/Ant Design 页签，不再使用自绘胶囊页签。
- 统一术语为“权限复核、接口凭据、配置发布、接入状态”等业务语言，删除“下一项”等无法指导
  操作的列名和无必要的说明框。
- 工作台、概览卡片、列表和详情页统一间距与容器规则；分页跟随表格底部；桌面、窄屏、200%
  缩放均纳入重叠和横向溢出检查。

### 3.2 管理闭环

- 组织：组织信息、部门树、用户组增删改、成员维护。
- 用户：创建、详情、停用/恢复、解锁、密码重置审批、MFA 状态、会话处置、账号来源。
- 权限：平台角色、应用角色、人员/部门/角色/全员访问范围、排除优先、临时权限。
- 应用：基础信息、开发/测试/生产 OIDC 环境、回调地址、上线检查、发布/停用、账号下发目标。
- 治理：审批待办、已批准待执行、失败重试、按应用/角色/部门/用户发起权限复核、批量保留。
- 审计：操作记录、完整性状态、归档与外部副本状态。
- 安全：风险触发 Turnstile；MFA 不默认强制，用户可自行开通并使用恢复码。

## 4. 自动化质量证据

`make verify` 已通过以下门禁：

- 128 个 HTTP 操作的契约和 OpenAPI 安全校验；Proto lint、breaking 和生成一致性。
- 全部 Go 单元测试和 race 测试；`go vet`、gosec、staticcheck、golangci-lint。
- `govulncheck` 可达漏洞为 0；gitleaks 当前树与历史扫描均未发现泄漏。
- Web lint、15 个测试文件共 65 个用例、生产构建和分包预算。
- Playwright：14 个项目用例中 9 个通过、5 个按设备条件预期跳过；覆盖登录、无应用空态、
  分组菜单、窄屏溢出、应用详情标准页签、卡片不重叠、键盘、缩放和 axe 可访问性。
- 生产 Compose、APISIX、PITR 脚本、可观测性静态策略、Helm lint/template 和供应链证据。

## 5. 生产发布证据

### 5.1 制品和回滚

- 本地使用 `goproxy.cn` 和 `registry.npmmirror.com` 构建 Linux AMD64 Server、Worker、迁移程序、
  Demo 和 Web；服务器只封装轻量 artifact 镜像。
- `BUILD-INFO`、`SHA256SUMS`、API 健康版本和 OCI revision/version 标签一致。
- Server/Worker 镜像：`sha256:ad4957bd686919df7b84e64d9dc0bbd06f743536e648033bce35a0d6fd1302a9`。
- Web 镜像：`sha256:dcadc8c4d80017e10b4327c91597b0f6bd6a0d66eec384f72bb927036756fef2`。
- 发布前回滚目录：`/opt/velora/prod/releases/20260824T150636Z-before-8dae77d`，包含 Compose、
  权限为 0600 的环境文件、旧镜像 ID/回滚标签和 206,613 字节 PostgreSQL 自定义格式备份。
- 新增迁移均为向前兼容。应用回滚保留新增表和审计历史，不执行破坏性降级。

### 5.2 运行状态

- PostgreSQL、Redis、Casdoor、Server、Worker、Web、Edge、OIDC Demo 全部 healthy。
- 公网 `home` 健康/就绪、`auth` OIDC discovery、`demo`、`spectra` 健康接口均返回 200。
- 数据库、缓存、消息、搜索、对象存储五项依赖均为 `UP`。
- 公网仅发布 80/443；HSTS、CSP、X-Frame-Options、nosniff、Referrer-Policy 和
  Permissions-Policy 已生效。
- 健康、备份、审计归档、证书续期四个 systemd timer 均 enabled/active。
- 最近备份和审计归档证据均为 passed、encrypted、signed、remote_copy；生产综合健康巡检
  于 `2026-08-24T15:21Z` 通过。
- `home/auth/demo` 证书到期时间均为 2026-11-20，自动续期已启用。

### 5.3 真实页面验收

- Carson 生产账号从 Velora 登录页成功登录并进入 `/home`，门户展示 Spectra 和接入示例；
  最近使用、待办中心和“开发安全”分类数据一致。
- 535px 窄视口下顶栏、应用卡片、待办、分类卡片无重叠或横向溢出。
- 退出登录成功回到 Velora 登录页，没有暴露 Casdoor JSON 或管理页面。
- Turnstile 默认不占据登录页，只在风险触发后显示；风险触发时官方控件能正常加载，登录按钮
  在取得 token 前禁用。

生产管理员账号触发 Turnstile 后的后台页面级验收仍需由人完成一次验证码点击。验证码属于
第三方人机确认，自动化不得代替用户完成；这不是代码或服务故障。仓库内管理员全流程和视觉
回归已经通过，生产写操作验收将在验证码确认后继续记录。

## 6. 发布过程中修复的运维缺陷

无源码服务器原先只保留 Compose，没有携带 PostgreSQL 初始化和入口脚本。依赖容器长期不重建
时不会暴露，重建 PostgreSQL 时会把不存在的宿主文件创建为目录并导致启动失败。本轮已：

1. 使用发布前数据库备份和持久卷恢复 PostgreSQL，数据未丢失。
2. 把两个运行时脚本安装为 root 所有、0755 文件，容器恢复 healthy。
3. 将生产 Compose 和两个脚本纳入构建制品与 `SHA256SUMS`。
4. 规定 Compose 只能使用 `--env-file prod.env`，禁止 shell `source` 解析 DSN。
5. 规定应用滚动发布使用 `--no-deps`，避免无计划重建 PostgreSQL、Redis 或 Casdoor。

## 7. 回滚步骤

应用回滚不删除数据库迁移：

```bash
cd /opt/velora/prod/runtime/compose
docker tag velora-rollback-server:before-897c821 velora-prod-server:latest
docker tag velora-rollback-worker:before-897c821 velora-prod-worker:latest
docker tag velora-rollback-web:before-897c821 velora-prod-web:latest
docker compose --env-file prod.env -f docker-compose.yml \
  up -d --no-build --no-deps --force-recreate server worker web
```

只有发生确认的数据损坏且业务方批准数据回退时，才停止写入并使用发布前
`velora.pgdump` 按 `docs/ops-backup.md` 恢复；正常代码回滚不得覆盖上线后的业务数据。

## 8. 尚未取得的金融级外部证据

| 项目 | 当前状态 | 达标条件 |
| --- | --- | --- |
| 多可用区高可用 | 单节点 | 至少双实例、数据库主备/托管 HA、故障切换实测 |
| 异地灾备 | 有备份与远端副本 | 独立区域恢复演练，记录真实 RPO/RTO |
| 国密 KMS/HSM | 有适配接口和启动门禁 | 接入合规设备/服务，完成密钥生命周期和故障演练 |
| WORM/SIEM | 有签名、加密、远端归档 | 受监管存储和 SIEM 实际接收、查询、告警证据 |
| 安全合规 | 自动化扫描通过 | 第三方渗透、等保/内控测评及整改闭环 |

以上项目不阻止当前受控单节点 V1 交付，但在对外宣称“金融级高可用/合规认证”前必须完成。
