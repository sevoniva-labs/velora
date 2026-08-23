-- +goose Up
CREATE TABLE IF NOT EXISTS user_role_exclusions (
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id varchar(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  review_item_id varchar(36) NOT NULL REFERENCES access_review_items(id) ON DELETE RESTRICT,
  reason varchar(500) NOT NULL,
  created_by varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id,role_id)
);
CREATE INDEX IF NOT EXISTS user_role_exclusions_org ON user_role_exclusions(organization_id,user_id);

-- +goose Down
DROP TABLE IF EXISTS user_role_exclusions;
