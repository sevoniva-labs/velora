-- +goose Up
CREATE TABLE IF NOT EXISTS data_field_policies (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    field_key varchar(200) NOT NULL,
    classification varchar(40) NOT NULL,
    owner varchar(160) NOT NULL,
    purpose varchar(300) NOT NULL,
    residency varchar(100) NOT NULL,
    retention_days integer NOT NULL,
    tags_json text NOT NULL DEFAULT '[]',
    mask_strategy varchar(40) NOT NULL,
    export_approval boolean NOT NULL DEFAULT false,
    watermark boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, field_key)
);
CREATE INDEX IF NOT EXISTS idx_data_field_policies_org_classification ON data_field_policies(organization_id, classification);

CREATE TABLE IF NOT EXISTS data_deletion_evidence (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    resource_type varchar(160) NOT NULL,
    resource_digest char(64) NOT NULL,
    field_keys_json text NOT NULL,
    reason varchar(500) NOT NULL,
    records_deleted bigint NOT NULL,
    deleted_at timestamptz NOT NULL,
    evidence_hash char(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, evidence_hash)
);
CREATE INDEX IF NOT EXISTS idx_data_deletion_evidence_org_created ON data_deletion_evidence(organization_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS data_deletion_evidence;
DROP TABLE IF EXISTS data_field_policies;
