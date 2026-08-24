# Velora Go SDK

包：`github.com/sevoniva-labs/velora/server/sdk/velora`。

- `OIDCClient`：Authorization Code、PKCE S256、Discovery/JWKS、Issuer/Audience/Expiry/Nonce。
- `ProvisioningHandler`：64 KiB、严格 JSON、HMAC、时钟偏差、幂等/版本和 challenge。
- 应用目录：通过接入包中的 `VELORA_DIRECTORY_BASE_URL` 和独立 `directory-token` 读取组织、部门和本应用已授权用户；接口契约由 OpenAPI 提供。
- `MemoryProvisioningStore` 只用于测试/Reference App；生产必须实现事务型 Store。

完整规范：[`docs/application-integration-guide.md`](../../../docs/application-integration-guide.md)。
