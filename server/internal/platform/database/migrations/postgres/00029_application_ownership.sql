-- +goose Up
ALTER TABLE portal_applications
  ADD COLUMN IF NOT EXISTS owner_user_id varchar(36) REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS owner_department_id varchar(36) REFERENCES departments(id);
CREATE INDEX IF NOT EXISTS portal_applications_owner_user ON portal_applications(organization_id, owner_user_id);
CREATE INDEX IF NOT EXISTS portal_applications_owner_department ON portal_applications(organization_id, owner_department_id);

-- +goose Down
DROP INDEX IF EXISTS portal_applications_owner_department;
DROP INDEX IF EXISTS portal_applications_owner_user;
ALTER TABLE portal_applications DROP COLUMN IF EXISTS owner_department_id, DROP COLUMN IF EXISTS owner_user_id;
