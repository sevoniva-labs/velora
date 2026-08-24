# Velora 安全与合规边界

## 已实现的工程控制

- OIDC Authorization Code + PKCE S256、State、Nonce、Issuer、Audience、签名和过期校验。
- 服务端 Session、Secure/HttpOnly/SameSite Cookie、CSRF、防开放重定向和登录限流。
- Turnstile 风险挑战、TOTP MFA 自助启用、恢复码和近期 MFA 高风险操作门禁。
- 组织隔离、角色权限、数据范围、三员分立起点、临时权限、审批和权限复核。
- 应用默认拒绝、部门/组/角色/人员范围、排除优先、账号停用和会话撤销。
- 账号下发 HMAC、时间窗、幂等、单调版本、可靠重试和失败告警状态。
- 应用目录接口使用应用绑定的独立 Bearer 凭据，仅返回本应用已授权用户。
- Secret 文件、短时加密 Handoff、一次性展示、轮换和日志脱敏。
- 审计哈希链、完整性检查、导出审批、供应链扫描和制品追溯。
- 备份、恢复、PITR 前置检查、生产配置门禁和可回滚发布。

## 强制接入规则

1. 新应用不得获取平台管理员 Session 或通配 API Token。
2. Client Secret、Provisioning Secret、Directory Token 不进入 Git、镜像、前端、工单、聊天和日志。
3. 业务应用不允许 JIT 默认建号；只有 Velora 已预配且状态为 `ACTIVE` 的账号可以登录。
4. 业务应用必须执行自己的业务 RBAC/数据权限，不得只依赖门户是否显示应用。
5. 回调地址精确匹配 HTTPS，不使用通配符、Query、Fragment 或 userinfo。
6. 停用账号必须撤销应用 Session 和 Token；重新启用不自动恢复历史高权限。

## 外部环境事项

以下事项必须由目标机构、云环境或第三方测评形成证据，目前不能宣称已经通过：

- 多节点高可用、异地灾备、两地三中心和批准的 RPO/RTO 演练。
- WORM 审计存储、SOC/SIEM 联调、告警值班和长期留存策略。
- 真实国密 KMS/HSM/密码机、TLCP、商密产品认证和密钥轮换演练。
- OceanBase、达梦、人大金仓、GaussDB 等目标数据库兼容认证。
- 等保、密评、金融监管验收、外部渗透测试和供应商认证。

这些能力已有适配边界、门禁或证据模板时，只能标记为“已准备”，不得标记为“已认证”。详细技术基线见 [`server/docs/security.md`](../server/docs/security.md) 和 [`server/docs/compliance-china-financial.md`](../server/docs/compliance-china-financial.md)。
