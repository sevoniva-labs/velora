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
# 导出 180 天前的记录（只导出，不删除在线记录）
./scripts/audit-archive.sh
# 自定义保留天数
AUDIT_RETENTION_DAYS=365 ./scripts/audit-archive.sh
# 归档目录（默认 ./backups/audit）
AUDIT_ARCHIVE_DIR=/srv/audit-archive ./scripts/audit-archive.sh
```

脚本行为：

1. `\copy` 导出 `occurred_at < cutoff` 的记录为 CSV（含 `prev_hash/event_hash`）；
2. 用 CSV 解析器校验导出行数等于数据库计数；
3. 生成 SHA-256 清单和带 cutoff、行数、schema 的元数据；
4. 始终保留在线记录。删除只能由验证 WORM receipt 并写入 `audit_chain_anchors` 的应用归档流程执行。

标准生产由 `velora-audit-archive.timer` 每日 03:30 执行 `run-production-audit-archive.sh`：导出当前时间前的完整快照，打包后使用 age 加密、OpenSSL 签名并上传腾讯 COS，结果写入 `runtime/evidence/audit-archive-last-success.json`，健康检查要求 48 小时内存在有效成功证据。

安装与验收：

```bash
install -m 0644 deployments/systemd/velora-audit-archive.{service,timer} /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now velora-audit-archive.timer
systemctl start velora-audit-archive.service
jq . /opt/velora/prod/runtime/evidence/audit-archive-last-success.json
```

该标准归档是独立、加密、签名的可恢复副本，但 `immutable=false`。未启用并验证 COS 对象锁、WORM receipt、外部链头锚定和 SIEM 前，不得宣称不可抵赖或金融级审计。

## 5. 安全事件联动

- 登录失败（凭据错误 / 账户锁定）记录 `LOGIN_FAILED` 审计动作
- Prometheus 告警 `LoginFailuresSpike`（5 分钟 ≥ 20 次）已配置
  （`deployments/monitoring/alerts.yml`）
- 运维可在 Grafana 按 `action="LOGIN_FAILED"`、`operator`（用户名）聚合定位暴力破解源

## 6. 注意事项

- 标准归档不删除在线数据，避免在没有 WORM receipt 时破坏审计链
- 防篡改链不替代备份：`scripts/backup-db.sh` 每日全量备份仍是恢复的最终保障
- 如需离线审计导出，直接查 `audit_logs` 表或使用归档 CSV
