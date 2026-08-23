# Velora Go SDK

包：`github.com/sevoniva-labs/velora/server/sdk/velora`。

- `OIDCClient`：Authorization Code、PKCE S256、Discovery/JWKS、Issuer/Audience/Expiry/Nonce。
- `ProvisioningHandler`：64 KiB、严格 JSON、HMAC、时钟偏差、幂等/版本和 challenge。
- `MemoryProvisioningStore` 只用于测试/Reference App；生产必须实现事务型 Store。

完整规范：`docs/application-integration-standard-v1.md`。

