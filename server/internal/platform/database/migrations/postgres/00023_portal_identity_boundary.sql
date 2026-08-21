-- +goose Up
ALTER TABLE portal_applications ADD COLUMN IF NOT EXISTS lifecycle_status varchar(32) NOT NULL DEFAULT 'PUBLISHED';
ALTER TABLE portal_applications ADD COLUMN IF NOT EXISTS published_at timestamptz NULL;
ALTER TABLE portal_applications ADD COLUMN IF NOT EXISTS published_by varchar(36) NOT NULL DEFAULT '';
ALTER TABLE portal_applications ADD COLUMN IF NOT EXISTS config_version bigint NOT NULL DEFAULT 1;
UPDATE portal_applications SET lifecycle_status='PUBLISHED', published_at=COALESCE(published_at, updated_at) WHERE launch_type='URL' AND status='ENABLED' AND lifecycle_status='PUBLISHED';
UPDATE portal_applications SET lifecycle_status='IDENTITY_PENDING', status='DISABLED' WHERE launch_type <> 'URL' AND lifecycle_status='PUBLISHED';

CREATE TABLE IF NOT EXISTS portal_application_identity_bindings (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  provider_key varchar(64) NOT NULL,
  protocol varchar(32) NOT NULL,
  provider_application_ref varchar(255) NOT NULL,
  public_client_id varchar(255) NOT NULL,
  issuer varchar(2048) NOT NULL DEFAULT '',
  redirect_uris_json text NOT NULL DEFAULT '[]',
  configuration_status varchar(32) NOT NULL DEFAULT 'PENDING',
  verification_status varchar(32) NOT NULL DEFAULT 'PENDING',
  verified_at timestamptz NULL,
  verified_by varchar(36) NOT NULL DEFAULT '',
  verification_error varchar(500) NOT NULL DEFAULT '',
  config_version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, application_id),
  UNIQUE (organization_id, provider_key, provider_application_ref)
);
CREATE INDEX IF NOT EXISTS idx_portal_identity_bindings_status ON portal_application_identity_bindings(organization_id, verification_status, updated_at);

CREATE TABLE IF NOT EXISTS portal_application_verifications (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  binding_id varchar(36) NOT NULL REFERENCES portal_application_identity_bindings(id) ON DELETE CASCADE,
  check_type varchar(64) NOT NULL,
  result varchar(32) NOT NULL,
  error_code varchar(128) NOT NULL DEFAULT '',
  evidence_json text NOT NULL DEFAULT '{}',
  verified_by varchar(36) NOT NULL DEFAULT '',
  occurred_at timestamptz NOT NULL DEFAULT now(),
  request_id varchar(128) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_portal_identity_verifications_app ON portal_application_verifications(organization_id, application_id, occurred_at DESC);

-- +goose Down
DROP TABLE IF EXISTS portal_application_verifications;
DROP TABLE IF EXISTS portal_application_identity_bindings;
ALTER TABLE portal_applications DROP COLUMN IF EXISTS config_version;
ALTER TABLE portal_applications DROP COLUMN IF EXISTS published_by;
ALTER TABLE portal_applications DROP COLUMN IF EXISTS published_at;
ALTER TABLE portal_applications DROP COLUMN IF EXISTS lifecycle_status;
