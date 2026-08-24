-- +goose Up
ALTER TABLE access_reviews ADD COLUMN scope_type varchar(32) NOT NULL DEFAULT 'ALL';
ALTER TABLE access_reviews ADD COLUMN scope_id varchar(100) NOT NULL DEFAULT '';
ALTER TABLE access_reviews ADD COLUMN scope_name varchar(200) NOT NULL DEFAULT '全部用户';
CREATE INDEX access_reviews_org_scope_idx ON access_reviews(organization_id,scope_type,scope_id);

-- +goose Down
DROP INDEX access_reviews_org_scope_idx ON access_reviews;
ALTER TABLE access_reviews DROP COLUMN scope_name;
ALTER TABLE access_reviews DROP COLUMN scope_id;
ALTER TABLE access_reviews DROP COLUMN scope_type;
