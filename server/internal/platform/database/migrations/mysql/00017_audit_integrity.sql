-- +goose Up
CREATE TABLE IF NOT EXISTS audit_chain_heads (
    scope varchar(36) NOT NULL PRIMARY KEY,
    sequence_no bigint NOT NULL,
    head_hash varchar(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
ALTER TABLE audit_logs ADD COLUMN sequence_no bigint NULL;
ALTER TABLE audit_logs ADD COLUMN prev_hash varchar(64) NULL;
ALTER TABLE audit_logs ADD COLUMN event_hash varchar(64) NULL;

-- +goose Down
ALTER TABLE audit_logs DROP COLUMN event_hash;
ALTER TABLE audit_logs DROP COLUMN prev_hash;
ALTER TABLE audit_logs DROP COLUMN sequence_no;
DROP TABLE IF EXISTS audit_chain_heads;
