# Web 与新后端接口适配说明

## 已完成

Web 的请求入口统一位于 `web/src/api/api.ts`，不再直接调用替换前的 Gin/GORM 路由。当前已适配：

- 认证：`/me`、`/auth/login`、`/auth/logout`、`/auth/password`。
- 门户：应用列表/详情/启动、最近访问、分类、标签、收藏。
- 管理：门户应用/分类/标签 CRUD、访问策略、审计日志、API Token、系统信息。
- 用户中心：资料、修改密码、管理员会话查看与吊销。
- Proto JSON 的 `snake_case` 响应统一在 `web/src/api/client.ts` 递归转换为页面使用的 `camelCase`。
- 本地 Compose 与 `server/configs/minimal.yaml` 默认允许 `http://localhost:5173` 与 `http://127.0.0.1:5173`，生产必须配置精确的 `VELORA_ALLOWED_ORIGINS`。

## 明确未接入的能力

当前产品范围没有邮件、待办、门户设置表和 Velora 自建 OIDC Provider。Web 不再访问这些不存在的旧 API：

- 邮件/待办读取显示空状态，写操作返回“尚未接入当前后端基座”，不会伪造成功。
- 门户设置页为只读固定基线；生产共享配置必须通过版本化配置发布，在线编辑需另建鉴权、审批和审计化配置服务。
- 管理端不再显示 Casdoor 同步和自建 OIDC Provider 客户端入口；Casdoor 本身未修改。

以上能力属于后续后端能力范围，不能将当前 Wave 1 宣称为邮件/待办或 OIDC Provider 功能完成。

## 验收命令

```bash
cd web
pnpm lint
pnpm test
pnpm build

cd ..
make test
```

真实登录后应能通过 Web 代理访问 `/api/v1/me`、`/api/v1/portal/applications`、`/api/v1/admin/portal/applications`、`/api/v1/admin/audit-logs`、`/api/v1/api-tokens` 和 `/api/v1/system/info`。
