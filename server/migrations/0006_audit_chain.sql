-- Velora 审计防篡改链（Phase C5）
--
-- 设计：
--   - audit_logs 增加 prev_hash / hash 两列：每条记录 hash = SHA256(prev_hash | action | resource |
--     resource_id | operator | ip | detail | created_at)，形成哈希链；篡改中间记录会破坏后续链。
--   - 历史行 prev_hash/hash 为空：由服务端启动时调用 BackfillChain 回填（Go 实现，与运行时
--     链哈希算法一致，避免 SQL digest 与 Go 时间格式差异导致校验失败）。
--   - 登录失败审计：登录失败（凭据错误 / 锁定拒绝）记 LOGIN_FAILED 动作，供安全告警（alerts.yml 已含
--     LoginFailuresSpike）。
--   - 保留策略：在线 180 天；超过 180 天的记录归档（导出）后删除。归档由运维脚本
--     scripts/audit-archive.sh 执行（见 docs/ops-audit.md）。

ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS prev_hash VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS hash      VARCHAR(64) NOT NULL DEFAULT '';

-- 索引：哈希链校验/归档查询
CREATE INDEX IF NOT EXISTS idx_audit_prev_hash ON audit_logs(prev_hash) WHERE prev_hash <> '';
