-- +goose Up
CREATE TABLE IF NOT EXISTS departments (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  parent_id varchar(36) NULL,
  department_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  sort_order int NOT NULL DEFAULT 0,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_departments_org_key (organization_id, department_key),
  KEY idx_departments_org_parent_sort (organization_id, parent_id, sort_order),
  KEY idx_departments_org_status (organization_id, status),
  CONSTRAINT fk_departments_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_departments_parent FOREIGN KEY (parent_id) REFERENCES departments(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS departments;
