-- +goose Up
ALTER TABLE roles ADD COLUMN data_scope_type varchar(30) NOT NULL DEFAULT 'SELF';

UPDATE roles SET data_scope_type = 'ORGANIZATION'
WHERE role_key IN ('system_admin', 'security_admin', 'auditor');

CREATE TABLE IF NOT EXISTS role_data_scope_departments (
  role_id varchar(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  department_id varchar(36) NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
  PRIMARY KEY (role_id, department_id)
);

CREATE INDEX IF NOT EXISTS idx_role_data_scope_departments_department
  ON role_data_scope_departments(department_id, role_id);

-- +goose Down
DROP TABLE IF EXISTS role_data_scope_departments;
ALTER TABLE roles DROP COLUMN data_scope_type;
