-- +goose Up
CREATE TABLE IF NOT EXISTS audit_chain_heads (
    scope varchar(36) PRIMARY KEY,
    sequence_no bigint NOT NULL,
    head_hash varchar(64) NOT NULL
);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS sequence_no bigint NULL;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS prev_hash varchar(64) NULL;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS event_hash varchar(64) NULL;

-- +goose Down
ALTER TABLE audit_logs DROP COLUMN IF EXISTS event_hash;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS prev_hash;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS sequence_no;
DROP TABLE IF EXISTS audit_chain_heads;
