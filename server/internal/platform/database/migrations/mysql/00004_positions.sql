-- +goose Up
CREATE TABLE IF NOT EXISTS positions (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  department_id varchar(36) NOT NULL,
  position_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  sort_order int NOT NULL DEFAULT 0,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_positions_org_key (organization_id, position_key),
  KEY idx_positions_org_department_sort (organization_id, department_id, sort_order),
  KEY idx_positions_org_status (organization_id, status),
  CONSTRAINT fk_positions_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_positions_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS positions;
