-- +goose Up
CREATE TABLE IF NOT EXISTS positions (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  department_id varchar(36) NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
  position_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, position_key)
);

CREATE INDEX IF NOT EXISTS idx_positions_org_department_sort ON positions(organization_id, department_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_positions_org_status ON positions(organization_id, status);

-- +goose Down
DROP TABLE IF EXISTS positions;
