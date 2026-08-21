-- +goose Up
CREATE TABLE IF NOT EXISTS access_reviews (
    id varchar(36) NOT NULL PRIMARY KEY,
    organization_id varchar(36) NOT NULL,
    reviewer_id varchar(36) NOT NULL,
    status varchar(32) NOT NULL,
    due_at datetime(6) NOT NULL,
    created_by varchar(36) NOT NULL,
    created_at datetime(6) NOT NULL,
    completed_at datetime(6) NULL,
    CONSTRAINT access_reviews_org_fk FOREIGN KEY (organization_id) REFERENCES organizations(id),
    CONSTRAINT access_reviews_reviewer_fk FOREIGN KEY (reviewer_id) REFERENCES users(id),
    CONSTRAINT access_reviews_creator_fk FOREIGN KEY (created_by) REFERENCES users(id)
);
CREATE INDEX access_reviews_org_status_idx ON access_reviews (organization_id, status, due_at);
CREATE TABLE IF NOT EXISTS access_review_items (
    id varchar(36) NOT NULL PRIMARY KEY,
    review_id varchar(36) NOT NULL,
    organization_id varchar(36) NOT NULL,
    user_id varchar(36) NOT NULL,
    login_name varchar(120) NOT NULL,
    role_key varchar(100) NOT NULL,
    decision varchar(32) NOT NULL,
    reason varchar(500) NOT NULL,
    decided_by varchar(36) NULL,
    decided_at datetime(6) NULL,
    created_at datetime(6) NOT NULL,
    CONSTRAINT access_review_items_unique_entitlement UNIQUE (review_id, user_id, role_key),
    CONSTRAINT access_review_items_review_fk FOREIGN KEY (review_id) REFERENCES access_reviews(id),
    CONSTRAINT access_review_items_org_fk FOREIGN KEY (organization_id) REFERENCES organizations(id),
    CONSTRAINT access_review_items_user_fk FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT access_review_items_decider_fk FOREIGN KEY (decided_by) REFERENCES users(id)
);
CREATE INDEX access_review_items_review_idx ON access_review_items (organization_id, review_id, decision);

-- +goose Down
DROP TABLE IF EXISTS access_review_items;
DROP TABLE IF EXISTS access_reviews;
