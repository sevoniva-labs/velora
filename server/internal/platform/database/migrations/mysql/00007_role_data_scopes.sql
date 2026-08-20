-- +goose Up
ALTER TABLE roles ADD COLUMN data_scope_type varchar(30) NOT NULL DEFAULT 'SELF';

UPDATE roles SET data_scope_type = 'ORGANIZATION'
WHERE role_key IN ('system_admin', 'security_admin', 'auditor');

CREATE TABLE IF NOT EXISTS role_data_scope_departments (
  role_id varchar(36) NOT NULL,
  department_id varchar(36) NOT NULL,
  PRIMARY KEY (role_id, department_id),
  KEY idx_role_data_scope_departments_department (department_id, role_id),
  CONSTRAINT fk_role_data_scope_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_role_data_scope_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS role_data_scope_departments;
ALTER TABLE roles DROP COLUMN data_scope_type;
