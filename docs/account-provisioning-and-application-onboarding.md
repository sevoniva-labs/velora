# Velora 统一账号与应用接入标准

状态：生产标准（Spectra 为首个参考实现）  
适用对象：平台管理员、应用开发者、运维与安全人员

## 1. 产品边界

| 能力 | 权威系统 | 说明 |
|---|---|---|
| 账号开通、停用、应用授权 | Velora | 管理员唯一日常操作入口，保存控制状态、版本和审计 |
| 密码、MFA、登录与 OIDC Token | Casdoor | 唯一认证源；不修改源码，不向普通用户暴露管理后台 |
| 应用内角色与业务权限 | 下游应用 | 只接受 Velora 签名下发；默认拒绝未知账号和未知角色 |
| 应用目录、可见范围、发布 | Velora | 门户可见性不能替代应用后端鉴权 |
| Client Secret | Casdoor 与下游应用 | 不进入 Velora 数据库、Git、镜像、前端或日志 |

Velora 不保存用户密码。管理员在 Velora 创建账号时，密码通过服务端受控链路只写入 Casdoor；Velora 仅保存 Casdoor 稳定 `subject` 和本地投影。Casdoor 不保存 Spectra 等下游应用角色。

## 2. 标准账号生命周期

### 入职/创建

1. 管理员在“账号与访问”创建账号，填写登录名、姓名、可选邮箱、初始密码和平台角色。
2. Velora 调用 Casdoor M2M API 建立身份，并读取稳定 `subject`。
3. Velora 在审计事务中保存本地投影；不得保存原始密码。
4. 管理员可同时开通 Spectra，Velora 将授权事件写入可靠消息表。
5. Worker 使用 HMAC 签名推送；Spectra 原子创建用户和角色。
6. 用户通过 Velora 登录并以 OIDC Authorization Code + PKCE 进入 Spectra。

### 转岗/授权变更

- 平台角色和应用角色必须分开管理。
- Velora 为每个用户的下发事件维护单调递增版本。
- Spectra 仅替换 `source=VELORA` 的角色，不覆盖本地 break-glass 授权。
- 未认证的角色名、未知应用编码和降序版本必须拒绝。

### 离职/停用

1. Velora 将 Casdoor 账号置为禁止登录并撤销 Casdoor Session/Token。
2. Velora 停用本地账号并撤销本地 Session/API Token。
3. Velora可靠下发 `user.disabled`；Spectra 停用用户并撤销 Session/API Token。
4. 任一外部步骤失败必须保留可重试记录和审计，不得返回伪成功。

重新启用账号不会自动恢复历史应用权限；管理员必须重新确认并授予。

## 3. 下游应用强制契约

### 3.1 OIDC

```text
Issuer=https://auth.sevoniva.com
Flow=Authorization Code
PKCE=S256
Scopes=openid profile email
```

应用必须校验签名、Issuer、Audience、Expiry、Nonce、一次性 State，并建立自己的 Host-only、Secure、HttpOnly、SameSite=Lax Session。不得 JIT 建号、默认授予角色或把 Token 放入 LocalStorage。

### 3.2 账号和权限下发

接口：`POST /api/v1/provisioning/events`

```text
X-Velora-Timestamp: <Unix seconds>
X-Velora-Signature: v1=<hex HMAC-SHA256(secret, timestamp + "." + raw-body)>
```

```json
{
  "schema_version": "1.0",
  "event_id": "UUID",
  "event_type": "user.entitlements.changed",
  "aggregate_version": 1,
  "occurred_at": "2026-08-22T00:00:00Z",
  "source": "velora",
  "user": {
    "subject": "stable-casdoor-subject",
    "issuer": "https://auth.sevoniva.com",
    "login_name": "carson",
    "display_name": "Carson",
    "email": "",
    "status": "ACTIVE"
  },
  "entitlements": { "application_code": "spectra", "roles": ["developer"] }
}
```

接收端必须：

- 限制 Body 为 64 KiB，严格 JSON，不接受未知字段；
- 使用恒定时间比较签名，只允许五分钟时钟偏差；
- 以 `event_id` 幂等，返回 `APPLIED`、`DUPLICATE` 或 `STALE`；
- 以 `aggregate_version` 防止旧事件覆盖新权限；
- 用户资料、角色、撤销会话、事件记录和审计在同一数据库事务完成；
- `DISABLED` 事件角色必须为空；
- 不接收密码、MFA、OIDC Token、Cookie 或任意 Client Secret。

### 3.3 可用性语义

下发为至少一次投递。网络失败或 5xx 留在可靠消息表重试；明确 4xx 视为契约/配置故障并告警。认证登录以应用本地已下发状态为准：没有用户或已停用时拒绝，不能临时放行。

## 4. 新应用接入清单

1. 确定不可变应用编码、生产域名、精确 Callback、负责人和角色目录。
2. 在 Casdoor 创建独立 OIDC Client；Secret 只写应用 Secret Manager。
3. 在 Velora 创建应用目录记录和访问策略。
4. 应用实现本文件 OIDC 与下发接口；每个应用使用独立 HMAC Secret。
5. Velora 注册应用编码、接收 URL 和允许角色；生产 URL 必须 HTTPS。
6. 先部署接收端，再启用 Velora Worker 投递。
7. 用测试账号完成创建、授权、登录、拒绝、停用、恢复、重复和乱序事件测试。
8. 记录镜像/提交、配置摘要、测试证据和回滚点后发布。

新增应用不得复制 Spectra Secret；必须使用新密钥并支持轮换窗口。

## 5. Spectra 参考实现

| 项目 | 值 |
|---|---|
| 应用编码 | `spectra` |
| 地址 | `https://spectra.sevoniva.com` |
| 登录入口 | `/api/v1/auth/oidc/login` |
| Callback | `/api/v1/auth/oidc/callback` |
| 下发接口 | `/api/v1/provisioning/events` |
| 角色目录 | `system_admin`、`security_admin`、`project_admin`、`developer`、`auditor`、`ci_service` |
| Secret 文件 | `/run/secrets/spectra_provisioning_secret` |

Spectra 的接收端迁移、实现和测试位于 Spectra `main` 的 `454d4fa`。真实生产验收结果只在执行后填写，不预填 PASS。

## 6. 运维、对账与告警

每日对账至少比较：Velora `external_subject/status/version/roles`、Casdoor `id/isForbidden/isDeleted`、应用 `subject/state/provisioning_version/roles`。差异只生成报告或重放现有事件，不允许以低版本覆盖高版本。

必须告警：可靠消息连续失败、积压超过五分钟、签名失败突增、时钟偏差、未知角色、同一用户持续 `STALE`、停用撤销失败。日志只记录事件 ID、用户内部 ID、版本、结果和请求 ID，不记录敏感载荷。

## 7. 发布与回滚

发布顺序：数据库备份 → 应用接收端/迁移 → Velora Server/Web → Velora Worker → 冒烟与真实账号验收。

回滚顺序：

1. 先停 Velora Worker，阻止新事件投递；
2. 恢复 Velora 上一镜像/提交和 Compose；数据库新增表列为向后兼容，不在事故窗口删除；
3. 恢复应用上一镜像；保留下发事件、账号、业务数据和审计；
4. 必要时在 Velora 停用应用入口，在 Casdoor 停用该 OIDC Client；
5. 修复后以更高版本重新下发，禁止手工降低版本或删除审计。

## 8. 生产验收记录模板

```text
Velora commit/image:
Application commit/image:
Test account:
Create identity + stable subject: PASS/FAIL
Provision APPLIED: PASS/FAIL
OIDC login + expected role: PASS/FAIL
Unknown/no entitlement denied: PASS/FAIL
Disable + all sessions/tokens revoked: PASS/FAIL
Restore + explicit regrant: PASS/FAIL
Duplicate event: PASS/FAIL
Stale event: PASS/FAIL
Invalid signature/skew/body/role: PASS/FAIL
Audit and reliable-message evidence: PASS/FAIL
Rollback point:
Executed at/by:
```
