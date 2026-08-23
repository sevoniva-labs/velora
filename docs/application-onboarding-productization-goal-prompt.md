# Velora 应用接入产品化 Goal 提示词

严格执行 `/Users/chuncheng/Downloads/code/velora/docs/application-onboarding-productization-plan.md`，完成 Phase 0 至 Phase 4 全部核心开发，使 Velora 的应用接入达到可复制、可自助、可审计、可回滚的产品级标准。

必须彻底移除 Spectra 专属硬编码，实现通用应用角色目录、通用 Provisioning Target 与 Dispatcher、多应用用户授权、一站式五步接入向导、Velora 后端受控自动编排 Casdoor、向导内自动审批、一次性凭据与短时加密 Handoff、`velora-connect` CLI、Operation/Outbox 与配置漂移对账、全链路验证和发布门禁、Go SDK、Reference App、统一接入文档、监控告警与回滚方案。保持 Casdoor 隐藏且不修改 Casdoor，不建设 Velora 自有 OIDC Provider；前端、后端、Worker、数据库迁移、测试、文档和生产配置全部同步完成。

全程只在 `main` 开发。开始前检查并提交当前有效修改作为回滚点；每完成一个可验证纵向切片立即执行相关测试，使用 Conventional Commits 提交并 push 远程，禁止长时间积累未推送代码。依赖、容器和构建源全部优先使用国内镜像。

必须以当前代码、真实接口和测试证据为准，禁止猜测、伪造结果、用 Mock 冒充生产验证、为 Spectra、Demo 或未来应用新增名称硬编码，禁止通过削弱安全校验换取流程简化。开发过程中自主排障并持续推进，不重复请求确认；只有确实缺少新的外部凭据，或需要执行任务范围外的重大变更时才报告阻塞。

完成后部署到 `ubuntu@175.27.250.53`，验证 `home.sevoniva.com`、`auth.sevoniva.com`、Spectra 和第二个通用 Demo 的真实接入、账号下发、角色、无权限空态、停用恢复、登录退出、Secret 不泄漏、回滚路径和生产健康；更新唯一权威接入文档与实施证据。只有全部完成定义和产品体验指标通过后才能结束 Goal。
