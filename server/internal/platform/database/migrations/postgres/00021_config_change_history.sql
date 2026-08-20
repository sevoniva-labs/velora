-- +goose Up
CREATE TABLE IF NOT EXISTS config_change_history (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    namespace varchar(200) NOT NULL,
    config_group varchar(200) NOT NULL,
    data_id varchar(300) NOT NULL,
    version bigint NOT NULL,
    expected_previous_version bigint NOT NULL,
    value_digest char(64) NOT NULL,
    value_ref varchar(500) NOT NULL,
    sensitive boolean NOT NULL DEFAULT false,
    created_by varchar(36) NOT NULL,
    approved_by varchar(36),
    approval_id varchar(36),
    state varchar(40) NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, namespace, config_group, data_id, version)
);
CREATE INDEX IF NOT EXISTS idx_config_change_history_org_updated ON config_change_history(organization_id, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS config_change_history;
