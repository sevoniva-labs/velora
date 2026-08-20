-- +goose Up
CREATE TABLE IF NOT EXISTS access_reviews (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id),
    reviewer_id varchar(36) NOT NULL REFERENCES users(id),
    status varchar(32) NOT NULL,
    due_at timestamptz NOT NULL,
    created_by varchar(36) NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    completed_at timestamptz NULL
);
CREATE INDEX IF NOT EXISTS access_reviews_org_status_idx ON access_reviews (organization_id, status, due_at);
CREATE TABLE IF NOT EXISTS access_review_items (
    id varchar(36) PRIMARY KEY,
    review_id varchar(36) NOT NULL REFERENCES access_reviews(id),
    organization_id varchar(36) NOT NULL REFERENCES organizations(id),
    user_id varchar(36) NOT NULL REFERENCES users(id),
    login_name varchar(120) NOT NULL,
    role_key varchar(100) NOT NULL,
    decision varchar(32) NOT NULL,
    reason varchar(500) NOT NULL DEFAULT '',
    decided_by varchar(36) NULL REFERENCES users(id),
    decided_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT access_review_items_unique_entitlement UNIQUE (review_id, user_id, role_key)
);
CREATE INDEX IF NOT EXISTS access_review_items_review_idx ON access_review_items (organization_id, review_id, decision);

-- +goose Down
DROP TABLE IF EXISTS access_review_items;
DROP TABLE IF EXISTS access_reviews;
