# 微信扫码登录接入与运维

本文说明 Velora 的微信开放平台网站应用扫码登录、账号绑定和解绑。它是生产配置手册，不是 Casdoor 使用入口。

## 产品边界

- 用户始终从 Velora 登录页或个人中心发起操作，回调固定为 `https://auth.sevoniva.com/_velora/wechat/callback`。
- 浏览器直接进入微信开放平台二维码页；不会进入 Casdoor 页面。
- Casdoor 仅在内网保存微信 AppSecret、用授权码换取微信身份，并把该身份绑定到既有 Casdoor 用户。
- Velora 保存“哪个企业用户已绑定”的状态、登录时间和审计记录，不保存 AppSecret、access token 或原始 openid。
- 微信未绑定企业账号时拒绝登录，不按昵称、手机号或邮箱自动注册、自动匹配或合并账号。
- 下游应用仍只信任 `auth.sevoniva.com` 的标准 OIDC；微信身份不会直接发给 Spectra 等应用。

## 申请前准备

申请方需要提供：

1. 已认证的微信开放平台主体及通过审核的“网站应用”。公众号、小程序和微信支付应用不能替代网站应用。
2. 网站应用的 AppID 和 AppSecret。AppSecret 只通过受控渠道写入 Casdoor，不写入代码、Git、Velora 环境变量或工单正文。
3. 在微信开放平台登记授权回调域 `auth.sevoniva.com`。平台实际回调 URL 为上面的固定 HTTPS 地址。
4. 应用名称、图标、官网、隐私政策和服务条款等审核材料；具体字段以微信开放平台当前审核页面为准。

身份平台提供：固定回调域、Velora 登录/绑定入口、Casdoor 内部 Provider 名称、联调窗口、验收记录和回滚操作。

## Casdoor 一次性配置

由身份平台管理员在内网管理面完成：

1. 新建 OAuth Provider，类型选择微信网站扫码登录，填入 AppID/AppSecret，名称建议 `wechat-open`。
2. 将 Provider 关联到 Velora 的 Casdoor Application。
3. 关闭 Application 的注册入口、自动注册及所有按邮箱、手机号或用户名自动关联规则。
4. 确认既有 Velora 用户仍以 Casdoor subject 作为唯一外部身份映射。
5. 不开放 Casdoor SPA、`/callback` 或管理 API 到公网；公网只保留 Velora 明确代理的微信 start/callback 与 OIDC 协议端点。

Casdoor 官方的 OAuth Provider 文档可用于核对 Provider 字段：[Casdoor OAuth providers](https://casdoor.org/docs/category/oauth)。Casdoor 升级前必须在预发布环境复测 `/api/login` 的 `signin` 与 `link` 返回契约。

## Velora 配置

功能默认关闭。具备正式凭据后配置：

```dotenv
VELORA_WECHAT_LOGIN_ENABLED=true
VELORA_WECHAT_APP_ID=wx...
VELORA_WECHAT_PROVIDER=wechat-open
VELORA_WECHAT_CALLBACK_URL=https://auth.sevoniva.com/_velora/wechat/callback
```

启动时强制检查 HTTPS 回调、固定回调路径、分布式缓存、Casdoor 私有会话桥和必要字段。任一项不完整都会拒绝以“已启用”状态启动。AppSecret 不属于 Velora 配置。

## 用户旅程

### 首次绑定

1. 用户先用企业账号密码登录 Velora。
2. 在“用户中心 → 微信”点击“绑定微信”。Velora 生成五分钟有效、一次性的绑定票据。
3. `auth.sevoniva.com` 校验票据和当前 Casdoor host-only 会话后跳转微信二维码。
4. 回调同时校验 state cookie、Redis 事务和 Casdoor 当前用户，再执行唯一绑定。
5. 绑定结果写入 Velora并记录 `identity.wechat.bind` 审计事件。

### 扫码登录

1. 登录页仅在后端能力开启时显示“微信扫码登录”。
2. state 与浏览器 HttpOnly cookie 必须一致，Redis 状态只能消费一次。
3. Casdoor 返回的 subject 必须已映射到 Velora 用户，且该用户在 Velora 有有效微信绑定记录；否则提示“尚未绑定”，不会创建账号。
4. 用户已启用 MFA 时继续在 Velora 页面完成验证码或恢复码校验。
5. Velora 创建门户会话，再通过一次性 POST 票据建立 `auth.sevoniva.com` 会话；随后进入 Spectra 等下游应用时沿用原有 OIDC 单点登录。

### 解绑

解绑操作要求已登录、CSRF 校验和限流；用户已启用 MFA 时还要求五分钟内完成 step-up。先解除 Casdoor 微信身份，再删除 Velora 状态，失败时不会报告成功。记录 `identity.wechat.unbind` 审计事件。

## 上线验收

- 未启用时，健康接口返回 `wechatLoginEnabled=false`，登录页和个人中心均无微信入口。
- state 缺失、不一致、超时或重复使用均失败；return path 只能是站内相对路径。
- 未绑定微信扫码后提示先登录绑定，Casdoor 与 Velora 均没有新增用户。
- 已绑定用户能登录门户；启用 MFA 的用户必须完成第二因素。
- 扫码后可从门户进入 Spectra，整个过程中不出现 Casdoor 页面或管理地址。
- 同一微信尝试绑定第二个账号失败；解绑后立即不能扫码登录。
- 审计中可查询 bind、unbind、federated login；日志和 URL 不含 code、票据、cookie、openid 或 AppSecret。

## 监控与告警

监控微信 start/callback 的 4xx/5xx 比率、Casdoor `/api/login` 延迟、Redis 错误、`WECHAT_STATUS_UNAVAILABLE`、绑定失败和重复 state。连续五分钟失败率异常时先关闭入口，账号密码和既有 OIDC 不受影响。

## 回滚

1. 设置 `VELORA_WECHAT_LOGIN_ENABLED=false` 并滚动重启 Velora；入口即时隐藏，既有密码/OIDC 会话不变。
2. 保留 `user_wechat_bindings` 数据便于恢复，不在紧急回滚中删除绑定或改动 Casdoor 用户。
3. 若怀疑 AppSecret 泄露，在微信开放平台轮换 Secret，只更新 Casdoor Provider。
4. 代码回滚到上一镜像；数据库 `00035` 为向后兼容的独立表，可保留。确需回退迁移时，确认功能关闭后执行 goose down。
