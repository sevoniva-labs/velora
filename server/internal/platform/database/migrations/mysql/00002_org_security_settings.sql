-- +goose Up
ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  ADD COLUMN IF NOT EXISTS description varchar(500) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS max_users int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_active_sessions int NOT NULL DEFAULT 0;

UPDATE organizations
  SET status = COALESCE(NULLIF(TRIM(status), ''), 'ACTIVE'),
      description = COALESCE(description, ''),
      max_users = COALESCE(max_users, 0),
      max_active_sessions = COALESCE(max_active_sessions, 0);

CREATE TABLE IF NOT EXISTS system_settings (
  organization_id varchar(36) NOT NULL,
  setting_key varchar(160) NOT NULL,
  setting_value text NOT NULL,
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_by varchar(120) NOT NULL DEFAULT '',
  PRIMARY KEY (organization_id, setting_key),
  KEY idx_system_settings_org (organization_id),
  KEY idx_system_settings_updated_at (updated_at),
  CONSTRAINT fk_system_settings_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS system_settings;
ALTER TABLE organizations
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS max_users,
  DROP COLUMN IF EXISTS max_active_sessions;
