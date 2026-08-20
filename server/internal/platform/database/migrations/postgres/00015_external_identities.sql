-- +goose Up
CREATE TABLE IF NOT EXISTS external_identities (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id),
    provider varchar(64) NOT NULL,
    subject varchar(512) NOT NULL,
    user_id varchar(36) NOT NULL REFERENCES users(id),
    created_by varchar(36) NOT NULL REFERENCES users(id),
    approval_id varchar(36) NOT NULL REFERENCES approval_requests(id),
    created_at timestamptz NOT NULL,
    last_authenticated_at timestamptz NULL,
    CONSTRAINT external_identities_provider_subject_uq UNIQUE (organization_id, provider, subject)
);
CREATE INDEX IF NOT EXISTS external_identities_user_idx ON external_identities (organization_id, user_id);

-- +goose Down
DROP TABLE IF EXISTS external_identities;
