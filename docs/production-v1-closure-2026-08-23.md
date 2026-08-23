# Velora + Spectra 生产 V1 收口记录

状态：生产已部署，真实账号完整浏览器验收通过
记录时间：2026-08-23（Asia/Shanghai）
Velora：`537d48f`（`main`）
Spectra：`0d44986`（`main`，运行制品 `c5e3c40`）

## 1. 产品边界

- Velora 是普通用户唯一登录与应用访问入口，管理用户、应用、授权策略、账号生命周期、下发状态与审计。
- Casdoor 不改源码，只承担密码/MFA、OIDC Client、授权码、Token、JWKS 与 UserInfo；公网 `auth` 域只开放协议白名单，根路径、登录页和管理 UI 返回 404。
- 下游应用负责 OIDC RP、本地 Session、业务 RBAC/数据权限和账号事件接收；不得 JIT 默认建号或默认授权。
- V1 是单机、standard profile 的企业生产首发，不宣称高可用、金融级、国密硬件认证或不可变审计认证。

## 2. 已关闭的生产阻断项

| 控制 | 结果与证据 |
|---|---|
| 身份域边界 | `auth` 根路径和 `/login` 为 404；Discovery/JWKS 为 200，Issuer 精确为 `https://auth.sevoniva.com` |
| 授权网关 | gateway session 绑定 user/org/source session；每次授权重验源 Session、租户、应用状态和 `CanAccess` |
| 登录 CSRF | 精确 Portal Origin、Fetch Metadata、30 秒单次票据、原浏览器 nonce 绑定；跨浏览器与跨站测试已加入门禁 |
| MFA 证明 | 只信任已验证的 AMR/ACR 或本系统实际验证结果，不从请求字段推断 |
| 联邦退出 | 认证来源与强度分离；OIDC 与 OIDC+MFA 都执行应用、本地 gateway、Casdoor 和 Portal 退出；Portal 退出请求携带 CSRF 与合法空 JSON 请求体 |
| SSRF | 当前 Casdoor-only 架构要求 Issuer 精确等于批准配置，并限制重定向和响应大小 |
| Casdoor 隐藏 | 普通用户不看到 Casdoor SPA；未修改 Casdoor 源码 |
| Spectra 登录 | `/login` 默认 Velora SSO；`/normal-login` 为隐藏应急入口，具备 CSRF、限流、锁定、TOTP/恢复码和审计 |
| Spectra SecretStore | 主密钥由 `SPECTRA_MASTER_KEY_FILE` 只读文件注入；生产告警已消除，密钥未进入 Git/镜像/环境变量 |
| 发布门禁 | Velora `make verify` 全通过；生产 Compose 静态门禁通过；Spectra Go test/vet 与前端生产依赖审计通过 |
| 资源与最小权限 | 自建服务只读根、drop capabilities、no-new-privileges、PID/CPU/内存边界；worker 有健康检查 |
| 备份恢复 | Velora/Casdoor 每日双库加密签名备份上传 COS；`20260822_224011` 隔离恢复与行数核对通过 |
| 健康监控 | 两分钟 systemd synthetic check 覆盖 Portal、OIDC、Spectra、容器、备份和证书；最新状态 `passed` |
| TLS 续期 | 三域已签发独立 Let's Encrypt 证书，过期日 2026-11-20；受控 timer 启用，三域 staging dry-run 全通过 |
| 标准审计归档 | 每日完整快照经 age 加密、OpenSSL 签名后上传 COS；独立状态证据进入健康检查。该副本不宣称 WORM |

## 3. Spectra 标准样板

Spectra 已部署 `c5e3c40`，实现默认 SSO、`/normal-login`、安全站内回跳、OIDC Code + PKCE S256、state/nonce、ID Token/UserInfo 校验、HMAC 账号事件、幂等版本控制、停用撤 Session/Token、无权限空态和联邦退出。后续应用必须逐项复用[应用接入规范 V1](./application-integration-standard-v1.md)，不能只复制登录按钮。

## 4. 自动化与线上验收结果

| 验收项 | 结果 |
|---|---|
| Velora `make verify` | PASS：test、race、vet、前端测试/lint/build、SAST/SCA、secret scan、供应链证据 |
| 生产 Compose 静态检查 | PASS：只发布 Edge 80/443，无默认凭据，身份域默认拒绝 |
| 线上 commit | Velora 源码与发布标识 `537d48f`，Web 运行制品含 `aceee63`，Edge 运行制品含 `537d48f`；Spectra 源码标识 `0d44986`、运行制品 `c5e3c40` |
| 线上健康 | Velora 8 个容器 running/healthy；健康证据 `status=passed`；Spectra API/worker running，公网 health 通过 |
| OIDC 入口 | PASS：Spectra `/login` → Velora，携带应用上下文、state、nonce、PKCE S256 和浏览器绑定 nonce |
| 证书续期 | PASS：三域正式签发、Nginx 校验/热加载、三域 staging renewal 成功 |
| 标准审计归档 | PASS：50 条审计事件已加密、签名并上传 COS；包清单、CSV 清单、签名和解密恢复抽检全部通过；证据 `audit-archive-restore-20260822T235819Z.json` |
| 真实账号浏览器登录/退出 | PASS：2026-08-23 使用 `carson` 从 Spectra `/login` 发起；Velora 正确显示“继续访问 Spectra”，Turnstile 通过后自动完成 session bridge、OIDC Code + PKCE、回调和 Spectra Session，10 秒观测窗口内到达 Spectra `/home`，未人工续跳；从 Spectra 退出后到达 `home.sevoniva.com/login?logged_out=1`，Portal logout HTTP 200，随后 `/api/v1/me` HTTP 401，无 Casdoor 页面或原始 JSON 暴露 |

## 5. 回滚点

- Velora 发布前回滚：`/opt/velora/prod/releases/20260822T231154Z-before-960982d`；nonce 增量回滚：`/opt/velora/prod/releases/20260822T232933Z-before-05f1216`。
- 审计归档增量回滚：`/opt/velora/prod/releases/20260822T235919Z-before-69f2ff5`；停用 `velora-audit-archive.timer` 并恢复该点源码即可，归档对象与在线审计数据均不删除。
- 登录跳转 Web 增量回滚：`/opt/velora/prod/releases/20260823T001137Z-before-aceee63`；联邦退出 Edge 增量回滚：`/opt/velora/prod/releases/20260823T002013Z-before-537d48f`。
- Spectra SSO 回滚：`/home/ubuntu/spectra-deploy/backup-20260823T0655Z-sso`；SecretStore 增量回滚：`/home/ubuntu/spectra-deploy/backup-20260822T232706Z-master-key`。
- 回滚优先恢复上一制品、Compose 和环境文件并按服务重建；保留 additive migration、账号、事件和审计数据。仅在数据损坏时按恢复手册使用加密备份，禁止普通回滚覆盖生产数据库。

## 6. 明确未完成且不得误报

- 当前单机无 HA；维护或主机故障会影响门户和身份协议。
- MFA 已支持用户自助，但按产品决定暂不对普通用户强制；管理员强制策略仍需组织策略确认。
- COS 桶对象锁为不可逆且可能需白名单，未经业务批准未开启；因此 WORM/SIEM、外部链头锚定和金融不可抵赖审计仍未认证。
- 腾讯云 KMS/HSM、国密硬件适配、SSE-KMS、异地容灾、外部渗透和合规测评未完成，不得对外宣称金融级。
- 监控已能本地判定失败；外部通知 Webhook 尚未提供，未证明五分钟内送达人。

以上未完成项不阻止 standard、单机、内部低风险应用的生产 V1，但必须进入风险台账，不能被页面或文案包装为已完成。
