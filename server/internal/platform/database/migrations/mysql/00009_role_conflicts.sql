-- +goose Up
CREATE TABLE IF NOT EXISTS role_conflict_rules (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  role_a varchar(100) NOT NULL,
  role_b varchar(100) NOT NULL,
  reason varchar(500) NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_role_conflict_rules_org_pair (organization_id, role_a, role_b),
  KEY idx_role_conflict_rules_org_status (organization_id, status),
  CONSTRAINT fk_role_conflict_rules_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT chk_role_conflict_rules_order CHECK (role_a < role_b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS role_conflict_rules;
