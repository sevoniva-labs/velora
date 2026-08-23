-- +goose Up
ALTER TABLE roles
  ADD COLUMN IF NOT EXISTS description varchar(500) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS status varchar(20) NOT NULL DEFAULT 'ACTIVE';
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_status_check;
ALTER TABLE roles ADD CONSTRAINT roles_status_check CHECK (status IN ('ACTIVE','DISABLED'));
CREATE INDEX IF NOT EXISTS roles_org_status ON roles(organization_id,status,role_key);

-- +goose Down
DROP INDEX IF EXISTS roles_org_status;
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_status_check;
ALTER TABLE roles DROP COLUMN IF EXISTS status;
ALTER TABLE roles DROP COLUMN IF EXISTS description;
