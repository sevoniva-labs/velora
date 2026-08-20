# 备份与恢复运维手册

> Phase A2 配套文档。备份与恢复是企业生产基线（RPO/RTO 定义见下），
> 本手册给出命令、调度建议与每月恢复演练流程。

## 1. RPO / RTO 目标

| 指标 | 目标 | 说明 |
| --- | --- | --- |
| RPO（数据丢失容忍） | ≤ 24h | 每日全量备份；如需更小 RPO，启用 PostgreSQL 连续归档（WAL）实现 PITR |
| RTO（恢复时间目标） | ≤ 1h | 单库恢复演练已验证；含基础设施重建建议 30–60 分钟 |

## 2. 备份命令

```bash
# 生产一致恢复点：同时备份 Velora 与 Casdoor（两个 URL 都必须提供）
DATABASE_URL='postgres://velora_app:***@postgres:5432/velora?sslmode=require' \
CASDOOR_DATABASE_URL='postgres://casdoor_app:***@postgres:5432/casdoor?sslmode=require' \
POSTGRES_CONTAINER=velora-prod-postgres ./scripts/backup-all-databases.sh

# 仅开发/单库场景才使用单库脚本
./scripts/backup-db.sh

# 自定义目录 / 保留天数 / 对象存储上传
BACKUP_DIR=/data/velora-backup BACKUP_RETENTION_DAYS=30 POSTGRES_CONTAINER=velora-prod-postgres ./scripts/backup-db.sh
BACKUP_S3=s3://velora-backup-prod ./scripts/backup-db.sh   # 需 s5cmd 或 aws cli
```

一致恢复点会生成 `velora_full_YYYYMMDD_HHMMSS.dump` 与
`casdoor_full_YYYYMMDD_HHMMSS.dump`（两者时间戳相同，PostgreSQL custom format）。

生产备份必须启用加密和校验清单。脚本使用 `age` 收件人文件加密，并在同目录生成
`.sha256` 清单；对象存储上传会同时上传两者。示例：

```bash
BACKUP_ENCRYPTION_REQUIRED=true \
BACKUP_ENCRYPTION_KEY_FILE=/secure/velora/backup/age-recipient.txt \
BACKUP_S3=s3://velora-backup-prod ./scripts/backup-db.sh
```

未安装 `age`、收件人文件不可读、或配置了 `BACKUP_S3` 但没有 `aws/s5cmd` 时脚本会失败，
不会报告“成功但未加密/未上传”。对象存储还必须由平台侧启用 SSE-KMS、版本控制、对象锁/保留策略和跨区域复制；脚本本身不假装提供这些能力。

## 3. 调度（cron 示例，生产主机）

```cron
# 每日 02:30 全量备份；日志保留 14 天
30 2 * * *  cd /opt/velora && ./scripts/backup-db.sh >> /var/log/velora-backup.log 2>&1
```

## 4. 恢复

```bash
# 恢复到 .env 指向的库（恢复前强制做保险备份，并要求确认）
RESTORE_DB_URL='postgres://velora_restore:***@postgres:5432/velora?sslmode=require' \
RESTORE_IDP_DB_URL='postgres://casdoor_restore:***@postgres:5432/casdoor?sslmode=require' \
RESTORE_CONFIRM=yes ./scripts/restore-all-databases.sh \
  backups/velora_full_20260101_020000.dump \
  backups/casdoor_full_20260101_020000.dump

# 恢复到指定新库
RESTORE_DB_URL='postgres://velora:velora@127.0.0.1:5433/velora?sslmode=disable' POSTGRES_CONTAINER=velora-prod-postgres \
  ./scripts/restore-db.sh backups/velora_full_20260101_020000.dump

# 加密备份：先把 age 私钥放到受控 Secret 路径
BACKUP_AGE_IDENTITY_FILE=/secure/velora/backup/age-identity.txt \
RESTORE_CONFIRM=yes ./scripts/restore-db.sh backups/velora_full_20260101_020000.dump.age
```

恢复后验证：

```bash
curl http://localhost:8080/healthz        # 服务健康
curl http://localhost:8080/api/v1/me      # 会话正常（需重新登录）
```

## 5. PITR（时间点恢复，可选增强）

若业务要求 RPO < 24h：

1. postgres 容器启用 WAL 归档（`archive_mode=on` + `archive_command` 写对象存储）；
2. 备份从"每日全量"升级为"每日全量 + 持续归档"；
3. 恢复时 `pg_basebackup` + `pg_archivecleanup` 回放到目标时间点。
4. 注意：casdoor 库与 velora 库同实例，PITR 会同时回放两者，需在恢复后检查 Casdoor 数据一致性。

## 5.1 审计归档

`scripts/audit-archive.sh` 只导出真实 `audit_logs` 字段并生成 CSV、SHA-256 清单和元数据，
不会直接删除在线记录。删除必须由应用归档服务完成：它要先把批次写入支持 immutable/WORM
retention 的对象存储，验证 receipt，再写 `audit_archive_receipts` 和
`audit_chain_anchors`，最后在同一事务删除。若没有这套 receipt/anchor 证据，禁止手工
`DELETE audit_logs`。

## 6. 每月恢复演练（必须执行）

> 备份不可验证 = 没有备份。每月至少一次演练，记录结果。

```bash
# 1. 取最近备份
ls -t backups/velora_full_*.dump | head -1

# 2. 创建临时库并恢复（与 velora 库隔离，不触碰生产）
docker exec velora-prod-postgres psql -U postgres \
  -c "CREATE DATABASE velora_drill OWNER postgres;"
docker cp <备份文件> velora-prod-postgres:/tmp/drill.dump
docker exec velora-prod-postgres pg_restore -U postgres -d velora_drill /tmp/drill.dump

# 3. 校验数据完整性
docker exec velora-prod-postgres psql -U postgres -d velora_drill -tAc \
  "SELECT count(*) FROM applications;"

# 4. 清理
docker exec velora-prod-postgres psql -U postgres -c "DROP DATABASE velora_drill;"
```

演练记录模板（建议入 `docs/ops-runbook.md`）：日期 / 备份文件 / 恢复耗时 / 表数量 / 结论。

## 7. 常见问题

| 问题 | 处理 |
| --- | --- |
| 本机无 pg_dump | 脚本自动回退到 `docker exec $POSTGRES_CONTAINER pg_dump`（默认 `velora-postgres`，生产为 `velora-prod-postgres`） |
| 恢复报"role does not exist" | 备份含 owner 信息，恢复时用 `--no-owner` 或确保同名角色存在 |
| 备份文件为空 | 检查 DATABASE_URL 是否指向正确库；容器场景确认对应 `POSTGRES_CONTAINER` 在运行 |
| 校验清单不匹配 | 立即停止恢复，重新从对象存储取同一备份和 `.sha256`；不要绕过校验 |
| 磁盘不足 | 备份目录与数据目录分离；开启 BACKUP_S3 异地备份 |
