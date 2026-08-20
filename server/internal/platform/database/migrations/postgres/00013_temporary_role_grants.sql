-- +goose Up
CREATE TABLE IF NOT EXISTS temporary_role_grants (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  role_id varchar(36) NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  requested_by varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  approval_id varchar(36) NOT NULL UNIQUE REFERENCES approval_requests(id) ON DELETE RESTRICT,
  reason varchar(500) NOT NULL,
  valid_from timestamptz NOT NULL,
  valid_until timestamptz NOT NULL,
  revoked_at timestamptz NULL,
  revoked_by varchar(36) NULL REFERENCES users(id) ON DELETE RESTRICT,
  revoke_reason varchar(500) NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT ck_temporary_role_grants_validity CHECK (valid_until > valid_from)
);
CREATE INDEX IF NOT EXISTS idx_temporary_role_grants_effective ON temporary_role_grants(user_id,valid_from,valid_until) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_temporary_role_grants_org_created ON temporary_role_grants(organization_id,created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS temporary_role_grants;
