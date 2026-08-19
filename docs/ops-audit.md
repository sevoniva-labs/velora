# Velora 审计防篡改链运维手册（Phase C5）

## 1. 机制

每条 `audit_logs` 记录带两个哈希字段：

- `prev_hash`：上一条记录的 `hash`（链前驱）
- `hash`：`SHA256(prev_hash | action | resource | resource_id | operator | ip | detail | created_at)`

时间统一截断到微秒（PostgreSQL `TIMESTAMPTZ` 精度），保证写入与校验算法一致。

任何中间记录被篡改（改 action / 删行 / 改字段），其后所有记录的 `prev_hash` 断链，
管理员执行完整性校验即可发现篡改位置。

## 2. 完整性校验

```bash
# 管理员（需 velora_admin）：
curl -H "X-CSRF-Token: $CSRF" -b cookies.txt \
  http://localhost:8080/api/v1/admin/audit-logs/verify
# → {"data":{"verified":true}}
# 篡改时：审计链校验失败：记录 #ID 哈希不匹配（可能被篡改）
```

建议加入 Prometheus 定时探测或月度巡检（配合 `docs/ops-backup.md` 的恢复演练）。

## 3. 历史数据回填

升级引入本机制时，历史记录 `prev_hash/hash` 为空。服务启动时自动执行
`audit.BackfillChain`（幂等，仅处理 `hash=''` 的记录），无需人工干预。

**注意**：如果曾用旧版本（纳秒精度）写入过 hash，需一次性重算：

```sql
UPDATE audit_logs SET prev_hash='', hash='';
-- 重启 server 自动回填
```

## 4. 保留策略与归档

| 阶段   | 保留时长 | 说明                                   |
|--------|----------|----------------------------------------|
| 在线   | 180 天   | API 可查询（默认）                     |
| 归档   | 3 年     | CSV 冷存储，满足合规取证               |

```bash
# 归档 180 天前的记录（导出 CSV 后删除，含 hash 链字段便于事后校验）
./scripts/audit-archive.sh
# 自定义保留天数
AUDIT_RETENTION_DAYS=365 ./scripts/audit-archive.sh
# 归档目录（默认 ./backups/audit，建议挂载对象存储）
AUDIT_ARCHIVE_DIR=/srv/audit-archive ./scripts/audit-archive.sh
```

脚本行为：

1. `\copy` 导出 `created_at < cutoff` 的记录为 CSV（含 `prev_hash/hash`）
2. 校验导出行数 == 待删行数，不一致则中止（防止误删）
3. `DELETE` 时保留最新一条作为链锚（`id < max(id)`），避免破坏后续新链

**cron 建议**（每日 03:30）：

```cron
30 3 * * * cd /opt/velora && ./scripts/audit-archive.sh >> /var/log/velora-audit-archive.log 2>&1
```

## 5. 安全事件联动

- 登录失败（凭据错误 / 账户锁定）记录 `LOGIN_FAILED` 审计动作
- Prometheus 告警 `LoginFailuresSpike`（5 分钟 ≥ 20 次）已配置
  （`deployments/monitoring/alerts.yml`）
- 运维可在 Grafana 按 `action="LOGIN_FAILED"`、`operator`（用户名）聚合定位暴力破解源

## 6. 注意事项

- 归档删除采用"先导出、校验、再删"三步，防止数据丢失
- 防篡改链不替代备份：`scripts/backup-db.sh` 每日全量备份仍是恢复的最终保障
- 如需离线审计导出，直接查 `audit_logs` 表或使用归档 CSV
