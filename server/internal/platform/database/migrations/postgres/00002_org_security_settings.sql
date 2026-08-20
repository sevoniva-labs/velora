-- +goose Up
ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  ADD COLUMN IF NOT EXISTS description varchar(500) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS max_users integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_active_sessions integer NOT NULL DEFAULT 0;

UPDATE organizations
  SET status = COALESCE(NULLIF(BTRIM(status), ''), 'ACTIVE'),
      description = COALESCE(description, ''),
      max_users = COALESCE(max_users, 0),
      max_active_sessions = COALESCE(max_active_sessions, 0);

CREATE TABLE IF NOT EXISTS system_settings (
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  setting_key varchar(160) NOT NULL,
  setting_value text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by varchar(120) NOT NULL DEFAULT '',
  PRIMARY KEY (organization_id, setting_key)
);

-- +goose Down
DROP TABLE IF EXISTS system_settings;
ALTER TABLE organizations
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS max_users,
  DROP COLUMN IF EXISTS max_active_sessions;
