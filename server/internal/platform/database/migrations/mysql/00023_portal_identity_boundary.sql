-- +goose Up
ALTER TABLE portal_applications ADD COLUMN lifecycle_status varchar(32) NOT NULL DEFAULT 'PUBLISHED';
ALTER TABLE portal_applications ADD COLUMN published_at timestamp(6) NULL;
ALTER TABLE portal_applications ADD COLUMN published_by varchar(36) NOT NULL DEFAULT '';
ALTER TABLE portal_applications ADD COLUMN config_version bigint NOT NULL DEFAULT 1;
UPDATE portal_applications SET lifecycle_status='IDENTITY_PENDING', status='DISABLED' WHERE launch_type <> 'URL' AND lifecycle_status='PUBLISHED';

CREATE TABLE IF NOT EXISTS portal_application_identity_bindings (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  application_id varchar(36) NOT NULL,
  provider_key varchar(64) NOT NULL,
  protocol varchar(32) NOT NULL,
  provider_application_ref varchar(255) NOT NULL,
  public_client_id varchar(255) NOT NULL,
  issuer varchar(2048) NOT NULL DEFAULT '',
  redirect_uris_json text NOT NULL,
  configuration_status varchar(32) NOT NULL DEFAULT 'PENDING',
  verification_status varchar(32) NOT NULL DEFAULT 'PENDING',
  verified_at timestamp(6) NULL,
  verified_by varchar(36) NOT NULL DEFAULT '',
  verification_error varchar(500) NOT NULL DEFAULT '',
  config_version bigint NOT NULL DEFAULT 1,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_portal_identity_app (organization_id, application_id),
  UNIQUE KEY uq_portal_identity_ref (organization_id, provider_key, provider_application_ref),
  KEY idx_portal_identity_status (organization_id, verification_status, updated_at),
  CONSTRAINT fk_portal_identity_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_identity_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_application_verifications (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  application_id varchar(36) NOT NULL,
  binding_id varchar(36) NOT NULL,
  check_type varchar(64) NOT NULL,
  result varchar(32) NOT NULL,
  error_code varchar(128) NOT NULL DEFAULT '',
  evidence_json text NOT NULL,
  verified_by varchar(36) NOT NULL DEFAULT '',
  occurred_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  request_id varchar(128) NOT NULL DEFAULT '',
  KEY idx_portal_identity_verifications_app (organization_id, application_id, occurred_at),
  CONSTRAINT fk_portal_verifications_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_verifications_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_verifications_binding FOREIGN KEY (binding_id) REFERENCES portal_application_identity_bindings(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS portal_application_verifications;
DROP TABLE IF EXISTS portal_application_identity_bindings;
ALTER TABLE portal_applications DROP COLUMN config_version;
ALTER TABLE portal_applications DROP COLUMN published_by;
ALTER TABLE portal_applications DROP COLUMN published_at;
ALTER TABLE portal_applications DROP COLUMN lifecycle_status;
