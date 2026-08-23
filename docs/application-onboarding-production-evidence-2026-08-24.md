# 应用接入产品化生产验收记录（2026-08-24）

## 结论

Phase 0–4 已在真实生产环境闭环。Velora 的应用接入不再依赖 Spectra 名称分支：Spectra 与第二个通用 Reference App 均使用同一角色目录、Provisioning Target、审批、Casdoor OIDC Client 编排、单次 Enrollment、五项门禁和发布状态机。Casdoor 未修改、管理面未向普通用户暴露，Velora 未建设自有 OIDC Provider。

## 发布版本

- Velora 生产应用制品：`e0d0066`；状态机与接入修复包含 `d8076f0`、`00162df`、`5574c79`、`bcd033f`、`b159e7d`。验收文档提交后仓库 `main` 为 `a32043d`，未改变生产二进制。
- Spectra `main`：`ab230a1`；新增标准接收路径和 challenge 的 `APPLIED / DUPLICATE / STALE` 语义。
- 生产域名：`home.sevoniva.com`、`auth.sevoniva.com`、`spectra.sevoniva.com`、`demo.sevoniva.com`。
- Velora Server 回滚镜像和时间戳保存在 `/opt/velora/prod/releases/*/rollback-stamp`；Spectra 回滚二进制和镜像保存在 `/home/ubuntu/spectra-releases/*/rollback-*`。

## 真实应用证据

| 项目 | Spectra | Reference App |
|---|---|---|
| Portal application | `spectra` | `velora-demo` |
| 生命周期 | `PUBLISHED / ENABLED` | `PUBLISHED / ENABLED` |
| 最终验证配置版本 | 16 | 14 |
| Provisioning Target | `HEALTHY`，key version 2 | `HEALTHY`，key version 1 |
| 当前版本自动门禁 | 5/5 PASS | 5/5 PASS |
| 接收路径 | `/api/v1/integrations/velora/provisioning` | `/api/v1/integrations/velora/provisioning` |
| 角色目录 | 六个 Spectra 业务角色 | `user`、`admin` |
| 访问策略 | 显式 `EVERYONE` | 显式 `EVERYONE` |

五项门禁均为真实 HTTPS 调用：访问策略、OIDC Discovery、签名 challenge、重复事件、低版本事件。未使用 Mock 代替生产结果。

Spectra 的 `carson` 投影为 `ACTIVE / managed_by=VELORA / provisioning_version=8`，下游角色为 `developer`；Velora 对应 entitlement 为 `ACTIVE / version=8 / ["developer"]`。可靠消息中 Spectra 的五条 `user.entitlements.changed` 和两条 `user.disabled` 均为 `PUBLISHED`，最大重试次数为 0。

## 安全与产品边界

- Reference App 的 Client Secret 与 Provisioning Secret 通过五分钟、单次 Enrollment Token 由 `velora-connect` 领取；CLI 以 `0600` 原子写入并通过 `doctor` 校验。证据只记录指纹，不记录 Secret。
- Protobuf JSON 的字符串型 `int64` 已由 CLI 兼容；领取中断可经过审批票据安全轮换新 Client Secret，不查询或复用旧值。
- Casdoor v1.762 的 `HTTP 200 + data:null` 未找到语义已兼容，避免把不存在的应用误判为已存在；生产已真实自动创建 Reference App Client。
- 未登录访问 Reference App 授权端点会由身份网关 302 到 `home.sevoniva.com/login?app=velora-demo...`；用户可见页面不出现 Casdoor。Demo 页面文案也仅使用“企业统一身份认证”。
- 已发布应用重新验签保持 `PUBLISHED`；已就绪应用重复提交不递增配置版本；检查证据不会被无意义状态写入作废。
- 无访问权限的用户接口按既有权限测试返回空应用集合而非 403；Spectra 对未预配或无业务角色用户失败关闭，`/normal-login` 仅保留为不展示的 break-glass 入口。
- 验收结束后，临时 Bootstrap API Token 和 Session 已全部撤销或删除；`carson` 仅保留普通 `user`，临时审批账号已禁用且无角色。领取目录和 `/run` 中的临时 Secret/响应文件已安全擦除，正式 Demo Secret 文件保持 `0400`、专用运行用户所有。

## 自动化验证

```text
Velora server: go test ./...                         PASS
Proto: make proto-check                             PASS
Contract: make contract                             PASS（118 HTTP operations）
Velora web: pnpm test --run                         PASS（48/48）
Velora web: pnpm lint                               PASS
Velora web: pnpm build                              PASS
Spectra: go test ./...                              PASS
Spectra production health                           HTTP 200
Velora health/readiness + Demo health               HTTP 200
Spectra production onboarding checks                PASS 5/5
Reference App production onboarding checks          PASS 5/5
Reference App published-state re-verification       PASS，仍为 PUBLISHED
```

## 部署与回滚

Velora 和 Spectra 均在本机构建 `linux/amd64` 静态二进制，再上传服务器封装镜像；基础镜像使用 DaoCloud 国内代理，Go 依赖使用 `goproxy.cn`。每次替换前保留上一镜像和二进制，数据库迁移为 additive，生产备份已加密、签名并上传既有 COS 桶。

回滚顺序：暂停新接入与下发 → 恢复上一 Server/Worker/目标应用镜像 → 保留 additive 表 → 验证健康、登录、应用列表和审计 → 对本次签发的 Client/Provisioning Secret 执行轮换。不得通过恢复旧 Secret、删除新表或 force-push 回滚。

## 运维观察项

持续监控 Target `DEGRADED`、Reliable Message 非 `PUBLISHED`、审批/Enrollment 失败、OIDC Discovery 和签名时钟偏差。Worker 对未配置可验证 WORM 归档的审计清理保持失败关闭；启用审计保留删除前，必须先配置并验证 WORM Archive Adapter。
