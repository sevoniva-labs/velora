-- +goose Up
CREATE TABLE IF NOT EXISTS user_assignments (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  department_id varchar(36) NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
  position_id varchar(36) NULL REFERENCES positions(id) ON DELETE RESTRICT,
  is_primary boolean NOT NULL DEFAULT false,
  valid_from timestamptz NOT NULL,
  valid_until timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_assignments_user_validity ON user_assignments(organization_id, user_id, valid_from, valid_until);
CREATE INDEX IF NOT EXISTS idx_user_assignments_department ON user_assignments(organization_id, department_id, user_id);
CREATE INDEX IF NOT EXISTS idx_user_assignments_position ON user_assignments(organization_id, position_id, user_id);

-- +goose Down
DROP TABLE IF EXISTS user_assignments;
