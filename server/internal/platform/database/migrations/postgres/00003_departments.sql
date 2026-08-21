-- +goose Up
CREATE TABLE IF NOT EXISTS departments (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  parent_id varchar(36) NULL REFERENCES departments(id) ON DELETE RESTRICT,
  department_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, department_key)
);

CREATE INDEX IF NOT EXISTS idx_departments_org_parent_sort ON departments(organization_id, parent_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_departments_org_status ON departments(organization_id, status);

-- +goose Down
DROP TABLE IF EXISTS departments;
