# Velora 应用接入排障

以向导的稳定错误码和 Request ID 为准，不粘贴 Secret、Token、Cookie、授权码或完整身份载荷。

| 错误/现象 | 处理 |
|---|---|
| `APPROVER_UNAVAILABLE` | 配置另一名启用的 `security_admin/system_admin`；申请人不能自批 |
| `MFA_REQUIRED/STEP_UP_REQUIRED` | 完成 MFA/二次验证后重试 |
| `ENROLLMENT_TOKEN_INVALID` | Token 已消费/过期；轮换并重新签发 |
| `OIDC_VERIFICATION_FAILED` | 核对 Discovery、Issuer、TLS、精确 Callback 和内外网解析 |
| `PROVISIONING_CHALLENGE_FAILED` | 运行 `doctor`，核对 Endpoint、Secret 文件、HMAC 和 NTP |
| `PROVISIONING_DUPLICATE_FAILED` | 相同 event_id+body 必须返回 `DUPLICATE` |
| `PROVISIONING_STALE_FAILED` | 低版本必须返回 `STALE`，不得覆盖高版本 |
| `CONFIG_VERSION_CONFLICT` | 刷新向导后基于最新版本操作 |
| Target `DEGRADED` | 修复目标应用后重跑检查，禁止关闭签名校验 |
| 登录暴露 Casdoor | 默认入口应跳 `home.sevoniva.com/login?app=<code>` |

回滚先暂停应用和新投递，保留数据与审计，恢复上一镜像；已签发或疑似泄露的 Secret 必须轮换。

