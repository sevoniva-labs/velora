-- +goose Up
-- Preserve the visibility of applications that intentionally relied on the
-- legacy implicit-public behavior before switching policy evaluation to
-- fail-closed. The deterministic ID keeps this migration idempotent.
INSERT INTO portal_access_policies(id,application_id,policy_type,value,created_at,updated_at)
SELECT md5('velora-default-everyone:' || a.id),a.id,'EVERYONE','',now(),now()
FROM portal_applications a
WHERE a.status='ENABLED'
  AND a.lifecycle_status='PUBLISHED'
  AND NOT EXISTS (
    SELECT 1 FROM portal_access_policies p WHERE p.application_id=a.id
  );

CREATE TABLE IF NOT EXISTS portal_application_roles (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  role_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  risk_level varchar(20) NOT NULL DEFAULT 'NORMAL' CHECK (risk_level IN ('NORMAL','PRIVILEGED','CRITICAL')),
  status varchar(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  config_version bigint NOT NULL DEFAULT 1 CHECK (config_version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, application_id, role_key)
);
CREATE INDEX IF NOT EXISTS idx_portal_application_roles_app_status
  ON portal_application_roles(organization_id, application_id, status, role_key);

CREATE TABLE IF NOT EXISTS portal_application_provisioning_targets (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  endpoint_url varchar(2048) NOT NULL,
  signing_algorithm varchar(32) NOT NULL DEFAULT 'HMAC-SHA256' CHECK (signing_algorithm='HMAC-SHA256'),
  secret_ref varchar(512) NOT NULL,
  secret_fingerprint varchar(128) NOT NULL,
  active_key_version bigint NOT NULL DEFAULT 1 CHECK (active_key_version > 0),
  previous_key_version bigint NULL,
  previous_valid_until timestamptz NULL,
  delivery_status varchar(32) NOT NULL DEFAULT 'DISABLED' CHECK (delivery_status IN ('DISABLED','PENDING','HEALTHY','DEGRADED')),
  last_success_at timestamptz NULL,
  last_failure_at timestamptz NULL,
  last_error_code varchar(128) NOT NULL DEFAULT '',
  config_version bigint NOT NULL DEFAULT 1 CHECK (config_version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, application_id)
);
CREATE INDEX IF NOT EXISTS idx_portal_provisioning_targets_status
  ON portal_application_provisioning_targets(organization_id, delivery_status, updated_at);

CREATE TABLE IF NOT EXISTS portal_application_onboarding_checks (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  config_version bigint NOT NULL CHECK (config_version > 0),
  check_type varchar(64) NOT NULL,
  result varchar(20) NOT NULL CHECK (result IN ('PENDING','PASSED','FAILED','SKIPPED')),
  error_code varchar(128) NOT NULL DEFAULT '',
  evidence_json text NOT NULL DEFAULT '{}',
  request_id varchar(128) NOT NULL DEFAULT '',
  verified_by varchar(36) NOT NULL DEFAULT '',
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_portal_onboarding_checks_app
  ON portal_application_onboarding_checks(organization_id, application_id, config_version, occurred_at DESC);

CREATE TABLE IF NOT EXISTS portal_application_onboarding_operations (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  operation_type varchar(64) NOT NULL,
  desired_version bigint NOT NULL CHECK (desired_version > 0),
  status varchar(32) NOT NULL CHECK (status IN ('PENDING','RUNNING','SUCCEEDED','FAILED','ACTION_REQUIRED')),
  idempotency_key varchar(128) NOT NULL,
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  provider_request_id varchar(128) NOT NULL DEFAULT '',
  result_summary_json text NOT NULL DEFAULT '{}',
  error_code varchar(128) NOT NULL DEFAULT '',
  next_retry_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz NULL,
  UNIQUE (organization_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_portal_onboarding_operations_retry
  ON portal_application_onboarding_operations(status, next_retry_at, created_at);

-- Seed the existing Spectra contract as data so all following code paths can
-- use the same catalog as future applications.
INSERT INTO portal_application_roles(id,organization_id,application_id,role_key,name,description,risk_level,status)
SELECT md5('spectra-role:' || a.id || ':' || seed.role_key),a.organization_id,a.id,seed.role_key,seed.name,seed.description,seed.risk_level,'ACTIVE'
FROM portal_applications a
CROSS JOIN (VALUES
  ('system_admin','系统管理员','管理 Spectra 全部系统能力','CRITICAL'),
  ('security_admin','安全管理员','管理 Spectra 安全能力','PRIVILEGED'),
  ('project_admin','项目管理员','管理 Spectra 项目','PRIVILEGED'),
  ('developer','开发人员','使用 Spectra 开发能力','NORMAL'),
  ('auditor','审计员','查看 Spectra 审计信息','PRIVILEGED'),
  ('ci_service','CI 服务账号','执行 Spectra 自动化任务','PRIVILEGED')
) AS seed(role_key,name,description,risk_level)
WHERE a.code='spectra'
ON CONFLICT (organization_id,application_id,role_key) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS portal_application_onboarding_operations;
DROP TABLE IF EXISTS portal_application_onboarding_checks;
DROP TABLE IF EXISTS portal_application_provisioning_targets;
DROP TABLE IF EXISTS portal_application_roles;
DELETE FROM portal_access_policies p
USING portal_applications a
WHERE p.application_id=a.id
  AND p.id=md5('velora-default-everyone:' || a.id)
  AND p.policy_type='EVERYONE'
  AND p.value='';
