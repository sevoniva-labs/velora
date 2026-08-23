-- +goose Up
CREATE TABLE IF NOT EXISTS user_role_exclusions (
  organization_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  role_id varchar(36) NOT NULL,
  review_item_id varchar(36) NOT NULL,
  reason varchar(500) NOT NULL,
  created_by varchar(36) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(user_id,role_id),
  KEY user_role_exclusions_org(organization_id,user_id),
  CONSTRAINT fk_role_exclusions_org FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_role_exclusions_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_role_exclusions_role FOREIGN KEY(role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_role_exclusions_item FOREIGN KEY(review_item_id) REFERENCES access_review_items(id) ON DELETE RESTRICT,
  CONSTRAINT fk_role_exclusions_actor FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE RESTRICT
);

-- +goose Down
DROP TABLE IF EXISTS user_role_exclusions;
