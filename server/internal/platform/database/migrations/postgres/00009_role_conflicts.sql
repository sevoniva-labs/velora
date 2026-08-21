-- +goose Up
CREATE TABLE IF NOT EXISTS role_conflict_rules (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  role_a varchar(100) NOT NULL,
  role_b varchar(100) NOT NULL,
  reason varchar(500) NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, role_a, role_b),
  CHECK (role_a < role_b)
);
CREATE INDEX IF NOT EXISTS idx_role_conflict_rules_org_status ON role_conflict_rules(organization_id, status);

-- +goose Down
DROP TABLE IF EXISTS role_conflict_rules;
