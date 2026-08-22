-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email varchar(320) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS identity_source varchar(32) NOT NULL DEFAULT 'LOCAL',
  ADD COLUMN IF NOT EXISTS external_subject varchar(512) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS provisioning_version bigint NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS users_external_subject_uq
  ON users(organization_id, identity_source, external_subject)
  WHERE external_subject <> '';

CREATE TABLE IF NOT EXISTS user_application_entitlements (
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  roles_json text NOT NULL DEFAULT '[]',
  status varchar(20) NOT NULL CHECK (status IN ('ACTIVE','DISABLED')),
  version bigint NOT NULL CHECK (version > 0),
  updated_by varchar(36) NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, application_id)
);
CREATE INDEX IF NOT EXISTS user_application_entitlements_app_status
  ON user_application_entitlements(application_id, status);

-- +goose Down
DROP TABLE IF EXISTS user_application_entitlements;
DROP INDEX IF EXISTS users_external_subject_uq;
ALTER TABLE users
  DROP COLUMN IF EXISTS provisioning_version,
  DROP COLUMN IF EXISTS external_subject,
  DROP COLUMN IF EXISTS identity_source,
  DROP COLUMN IF EXISTS email;
