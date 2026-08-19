# i18n 与合规（Phase D5 说明）

## 现状：文案已集中，切换层留待扩展

| 层 | 现状 | 扩展点 |
|---|---|---|
| 前端枚举文案 | `web/src/labels.ts` 集中 8 组映射（SSO 类型/应用状态/健康状态/策略类型/优先级等） | 将 `Record<X, string>` 改为 `Record<Locale, Record<X, string>>`，按当前 locale 取值 |
| 前端通用文案 | 各页面内联中文 | 逐页抽到 `web/src/i18n/zh-CN.ts` 资源文件，`useI18n()` hook 读取 |
| 后端错误消息 | `server/internal/platform/errs` 统一中文 message（伴随稳定错误码 `A01xxx` 等） | 客户端据错误码自行本地化，后端 message 作为兜底；不建议运行时切换（会破坏审计与日志可读性） |
| 后端审计/动作名 | 英文常量（`LOGIN`、`TODO_UPSERT`） | 已是稳定枚举，天然 i18n 中立 |

**切换机制（未实现，属 Phase 3）**：`Accept-Language` 头 → 前端 `Intl` 兜底 → `localStorage` 覆盖。当前产品为中文内部系统，切换层在需要时按上表扩展即可。

## 合规：数据可携带权（已完成）

- `GET /api/v1/admin/users/:id/export`（管理员）：导出指定用户在 Velora 侧的全量数据——
  收藏、访问记录、待办、邮件元数据、审计操作记录（上限 5000 条），JSON 附件下载。
- 审计记录 `USER_DATA_EXPORT`（operator/目标用户/时间），导出行为本身可追溯。
- 邮件正文不随导出（体积与隐私权衡）；删除用户前建议先导出留证。

## 相关

- 审计链校验与归档：`docs/ops-audit.md`
- 备份与恢复：`docs/ops-backup.md`
- 部署与密钥轮换：`docs/ops-deploy.md`、`scripts/rotate-secrets.sh`
