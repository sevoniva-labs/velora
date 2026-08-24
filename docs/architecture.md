# Velora 架构与产品边界

## 1. 产品定位

Velora 是企业统一应用门户和身份治理控制面。员工从 Velora 登录、查看并启动有权使用的应用；管理员在 Velora 管理组织、人员、应用角色、使用范围、账号下发、审批、上线检查和审计。

Casdoor 是隐藏的认证协议引擎，负责密码、MFA、OIDC Client、授权码、Token、JWKS 和 UserInfo。Velora 不修改 Casdoor 源码，不把 Casdoor 管理后台作为日常操作入口，也不建设自有 OIDC Provider。

业务应用负责自己的 Session、业务角色和数据权限。门户可见不等于业务授权；业务应用必须默认拒绝未知账号、停用账号和未知角色。

## 2. 责任边界

| 能力 | Velora | Casdoor | 业务应用 |
|---|---|---|---|
| 登录页面和用户入口 | 负责 | 不暴露 | 默认跳转 Velora |
| 密码、MFA、授权码和 Token | 编排和策略 | 负责 | 按标准校验 |
| 组织、人员、应用授权 | 权威控制面 | 保存认证身份 | 接收本应用投影 |
| OIDC Client | 自动申请、审批和交付 | 创建和签发 | 安全保存 Client Secret |
| 应用角色 | 定义、审批和下发 | 不负责 | 执行最终业务授权 |
| 账号同步 | 可靠推送并提供对账接口 | 不负责 | 幂等接收、默认拒绝 |
| 应用发布与停用 | 负责 | 同步 Client 状态 | 提供健康和回滚能力 |
| Casdoor 管理入口 | 仅身份管理员应急使用 | 提供 | 不使用 |

## 3. 登录链路

1. 用户从业务应用或 Velora 发起统一登录。
2. 未登录时只展示 `home.sevoniva.com` 的 Velora 登录页。
3. Velora 在服务端与 Casdoor 完成认证和短时 Session Bridge。
4. Casdoor 标准授权端点签发一次性 Authorization Code。
5. 业务应用使用 PKCE Verifier 和 Client Secret 换取 Token，校验签名、Issuer、Audience、Nonce 和过期时间。
6. 业务应用确认本地账号为 `ACTIVE` 且存在本应用角色后建立 Host-only Session。

浏览器地址栏可能出现协议域名 `auth.sevoniva.com`，但不得出现 Casdoor 登录页、管理页、内部容器地址或原始 API JSON。

## 4. 账号与组织链路

Velora 同时提供两条互补通道：

- 推送：用户授权、角色变化和停用事件通过签名 Provisioning Webhook 至少一次投递；用于实时变更。
- 拉取：每应用独立目录凭据读取组织、部门和本应用已授权用户；用于首次全量、定期对账和故障恢复。

拉取接口不会返回全组织未授权用户、平台权限、密码、MFA、Cookie 或 Token。应用停用或账号下发密钥轮换后，目录凭据同步失效。

## 5. 部署边界

当前生产形态是受控单机 Compose，适合现阶段企业级首发和应用接入验证。数据库、缓存、消息、搜索和对象存储均通过适配层接入。多节点、高可用、异地灾备、真实国密 KMS/HSM、WORM/SIEM 和外部合规测评需要目标机构提供基础设施后单独验收，不能由代码测试替代。

后端详细架构见 [`server/docs/architecture.md`](../server/docs/architecture.md)，接入操作见[《应用接入手册》](./application-integration-guide.md)。
