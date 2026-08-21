-- +goose Up
CREATE TABLE IF NOT EXISTS external_identities (
    id varchar(36) NOT NULL PRIMARY KEY,
    organization_id varchar(36) NOT NULL,
    provider varchar(64) NOT NULL,
    subject varchar(512) NOT NULL,
    user_id varchar(36) NOT NULL,
    created_by varchar(36) NOT NULL,
    approval_id varchar(36) NOT NULL,
    created_at datetime(6) NOT NULL,
    last_authenticated_at datetime(6) NULL,
    CONSTRAINT external_identities_provider_subject_uq UNIQUE (organization_id, provider, subject),
    CONSTRAINT external_identities_org_fk FOREIGN KEY (organization_id) REFERENCES organizations(id),
    CONSTRAINT external_identities_user_fk FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT external_identities_creator_fk FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT external_identities_approval_fk FOREIGN KEY (approval_id) REFERENCES approval_requests(id)
);
CREATE INDEX external_identities_user_idx ON external_identities (organization_id, user_id);

-- +goose Down
DROP TABLE IF EXISTS external_identities;
