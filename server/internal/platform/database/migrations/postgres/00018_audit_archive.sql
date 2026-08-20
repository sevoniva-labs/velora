-- +goose Up
CREATE TABLE IF NOT EXISTS audit_archive_receipts (
    id varchar(36) PRIMARY KEY,
    provider varchar(120) NOT NULL,
    object_key varchar(512) NOT NULL,
    version_id varchar(200) NOT NULL,
    content_hash varchar(64) NOT NULL,
    event_count bigint NOT NULL,
    archived_at timestamptz NOT NULL,
    immutable_until timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS audit_chain_anchors (
    scope varchar(36) PRIMARY KEY,
    sequence_no bigint NOT NULL,
    head_hash varchar(64) NOT NULL,
    archived_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS audit_chain_anchors;
DROP TABLE IF EXISTS audit_archive_receipts;
