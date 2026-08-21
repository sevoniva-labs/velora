-- +goose Up
CREATE TABLE IF NOT EXISTS user_assignments (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  department_id varchar(36) NOT NULL,
  position_id varchar(36) NULL,
  is_primary boolean NOT NULL DEFAULT false,
  valid_from timestamp(6) NOT NULL,
  valid_until timestamp(6) NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_user_assignments_user_validity (organization_id, user_id, valid_from, valid_until),
  KEY idx_user_assignments_department (organization_id, department_id, user_id),
  KEY idx_user_assignments_position (organization_id, position_id, user_id),
  CONSTRAINT fk_user_assignments_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_assignments_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_assignments_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE RESTRICT,
  CONSTRAINT fk_user_assignments_position FOREIGN KEY (position_id) REFERENCES positions(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS user_assignments;
