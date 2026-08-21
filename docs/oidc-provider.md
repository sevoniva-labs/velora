# 已废止：Velora 自建 OIDC Provider

状态：Deprecated

Velora 不再作为 OIDC Provider，也不再对外提供 `/oidc/authorize`、`/oidc/token`、`/oidc/userinfo` 或 `/oidc/jwks` 作为应用接入协议。

当前生产边界：

- Velora：应用目录、访问策略、启动入口、接入流程、发布与审计。
- Casdoor：OIDC Provider、账号、MFA、SSO Session、Client 和 Token。
- 业务应用：OIDC Relying Party、Token 校验、本地 Session 和业务 RBAC。

所有新应用必须按照[《Velora 应用 OIDC 接入指南》](./application-oidc-integration-guide.md)接入，Issuer 固定使用 `https://auth.sevoniva.com`。

禁止继续开发或部署 Velora 自建 OIDC Provider；旧的 `VELORA_OIDC` 接入类型只作为历史数据标识，不得用于新应用。
