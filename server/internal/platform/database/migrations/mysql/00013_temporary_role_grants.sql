-- +goose Up
CREATE TABLE IF NOT EXISTS temporary_role_grants (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  role_id varchar(36) NOT NULL,
  requested_by varchar(36) NOT NULL,
  approval_id varchar(36) NOT NULL,
  reason varchar(500) NOT NULL,
  valid_from timestamp(6) NOT NULL,
  valid_until timestamp(6) NOT NULL,
  revoked_at timestamp(6) NULL,
  revoked_by varchar(36) NULL,
  revoke_reason varchar(500) NULL,
  created_at timestamp(6) NOT NULL,
  UNIQUE KEY uk_temporary_role_grants_approval (approval_id),
  KEY idx_temporary_role_grants_effective (user_id,revoked_at,valid_from,valid_until),
  KEY idx_temporary_role_grants_org_created (organization_id,created_at),
  CONSTRAINT fk_temporary_role_grants_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT,
  CONSTRAINT fk_temporary_role_grants_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT fk_temporary_role_grants_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE RESTRICT,
  CONSTRAINT fk_temporary_role_grants_requester FOREIGN KEY (requested_by) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT fk_temporary_role_grants_approval FOREIGN KEY (approval_id) REFERENCES approval_requests(id) ON DELETE RESTRICT,
  CONSTRAINT fk_temporary_role_grants_revoker FOREIGN KEY (revoked_by) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT ck_temporary_role_grants_validity CHECK (valid_until > valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS temporary_role_grants;
