# S3 存储能力契约

## 目标

脚手架通过 AWS SDK for Go v2 使用统一 S3 协议适配层，支持 Generic S3、AWS S3、MinIO、Ceph RGW、阿里 OSS、腾讯 COS、华为 OBS 等 `Profile`。产品名称只用于配置路由，不代表兼容性认证。

## Fail-closed 规则

- `New` 只启用基础对象读写；高级能力保持 `Unknown`。
- 分片恢复、校验和、SSE-S3、SSE-KMS、版本、Object Lock、Retention、Legal Hold、受限预签名和临时凭证，必须由目标环境的契约测试逐项证明。
- 统一 `GovernanceStore` 提供受控对象写入、版本读取、版本控制、Retention 和 Legal Hold；写入参数会按请求能力逐项校验并 fail-closed。
- STS/临时凭证通过 `TemporaryCredentialIssuer` 适配位接入，因为不同 S3 产品的 STS 地址、签名和最小权限模型不一致。
- 契约必须标为 `Target-tested`，包含目标标识、测试时间、不可变证据引用和证据文件 SHA-256 摘要。
- 未通过的能力不可通过厂商名称、配置别名或前端开关绕过。

## 启用方式

目标环境完成契约测试后，通过 `storage.NewWithCapabilityContract` 注入契约。契约测试应使用专用测试桶和最小权限账号，覆盖成功、权限拒绝、超时、重试、清理和审计记录；不能把单元测试当作厂商认证。

```go
store, err := storage.NewWithCapabilityContract(ctx, cfg.Storage, targetContract)
```

当前仓库只提供协议能力边界和验证入口，AWS、MinIO、Ceph、OSS、COS、OBS 以及国产对象存储的目标版本、签名行为、STS、KMS/HSM、Object Lock 和灾备结果仍属于 `Not certified`，必须在实际交付环境留存证据后再升级标签。

## Upload quarantine

`QuarantineController` enforces the common regulated-upload path: server-side size and content-type checks, SHA-256 binding, quarantine storage, malware-scan evidence, atomic state transitions, governed promotion, and cleanup retry. The record store must provide an atomic compare-and-swap implementation backed by the application database; the in-memory fakes in tests are not a production store.

The malware scanner and quarantine object store are adapter slots. A clean local test is not evidence that a target S3 provider, scanner, KMS/HSM, or retention policy is certified. Promotion always calls `GovernanceStore.PutGoverned`, so untested S3 capabilities still fail closed.
