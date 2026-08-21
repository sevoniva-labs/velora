-- +goose Up
CREATE TABLE IF NOT EXISTS organizations (
  id varchar(36) PRIMARY KEY,
  org_key varchar(100) NOT NULL UNIQUE,
  name varchar(200) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id),
  login_name varchar(120) NOT NULL,
  display_name varchar(200) NOT NULL DEFAULT '',
  password_hash text NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  must_change_password boolean NOT NULL DEFAULT true,
  failed_login_count integer NOT NULL DEFAULT 0,
  locked_until timestamptz NULL,
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, login_name)
);
CREATE INDEX IF NOT EXISTS idx_users_org_status ON users(organization_id, status);

CREATE TABLE IF NOT EXISTS roles (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  role_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, role_key)
);
CREATE TABLE IF NOT EXISTS permissions (
  id varchar(36) PRIMARY KEY,
  permission_key varchar(160) NOT NULL UNIQUE,
  name varchar(200) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS user_roles (
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id varchar(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);
CREATE TABLE IF NOT EXISTS role_permissions (
  role_id varchar(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id varchar(36) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS password_history (
  id varchar(36) PRIMARY KEY,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  password_hash text NOT NULL,
  changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_password_history_user_time ON password_history(user_id, changed_at DESC);

CREATE TABLE IF NOT EXISTS sessions (
  id varchar(36) PRIMARY KEY,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash varchar(64) NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  client_ip varchar(64) NOT NULL DEFAULT '',
  user_agent varchar(512) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Machine/API identities. Only a SHA-256 token hash is persisted; the raw token
-- is returned once at creation time.
CREATE TABLE IF NOT EXISTS api_tokens (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id),
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name varchar(120) NOT NULL,
  token_prefix varchar(24) NOT NULL,
  token_hash varchar(64) NOT NULL UNIQUE,
  scopes_json text NOT NULL DEFAULT '[]',
  expires_at timestamptz NULL,
  last_used_at timestamptz NULL,
  revoked_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_tokens_expiry ON api_tokens(expires_at);

CREATE TABLE IF NOT EXISTS audit_logs (
  id varchar(36) PRIMARY KEY,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  request_id varchar(80) NOT NULL DEFAULT '',
  organization_id varchar(36) NULL,
  actor_id varchar(36) NULL,
  actor_name varchar(200) NOT NULL DEFAULT '',
  action varchar(200) NOT NULL,
  resource_type varchar(100) NOT NULL DEFAULT '',
  resource_id varchar(100) NOT NULL DEFAULT '',
  result varchar(20) NOT NULL DEFAULT 'SUCCESS',
  client_ip varchar(64) NOT NULL DEFAULT '',
  details_json text NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_occurred_at ON audit_logs(occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_time ON audit_logs(organization_id, occurred_at DESC);

-- Idempotency storage is DB-backed so retries across pods behave consistently.
CREATE TABLE IF NOT EXISTS idempotency_records (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  scope varchar(120) NOT NULL,
  idem_key varchar(160) NOT NULL,
  request_hash varchar(64) NOT NULL,
  state varchar(20) NOT NULL DEFAULT 'PROCESSING',
  response_status integer NULL,
  response_body bytea NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  UNIQUE (organization_id, scope, idem_key)
);
CREATE INDEX IF NOT EXISTS idx_idem_expiry ON idempotency_records(expires_at);

-- Local reliable-message table committed atomically with business state.
CREATE TABLE IF NOT EXISTS reliable_messages (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NULL,
  topic varchar(200) NOT NULL,
  event_key varchar(200) NOT NULL DEFAULT '',
  event_type varchar(160) NOT NULL,
  ordering_key varchar(200) NOT NULL DEFAULT '',
  tag varchar(128) NOT NULL DEFAULT '',
  deliver_at timestamptz NULL,
  payload_json text NOT NULL,
  headers_json text NOT NULL DEFAULT '{}',
  status varchar(20) NOT NULL DEFAULT 'PENDING',
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz NULL,
  last_error varchar(1000) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_reliable_messages_pending ON reliable_messages(status, next_attempt_at, created_at);

CREATE TABLE IF NOT EXISTS consumed_messages (
  consumer_group varchar(160) NOT NULL,
  event_id varchar(200) NOT NULL,
  organization_id varchar(36) NULL,
  topic varchar(200) NOT NULL,
  event_type varchar(160) NOT NULL,
  body_hash char(64) NOT NULL,
  provider_message_id varchar(200) NOT NULL DEFAULT '',
  consumed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_group, event_id)
);
CREATE INDEX IF NOT EXISTS idx_consumed_messages_at ON consumed_messages(consumed_at);

CREATE TABLE IF NOT EXISTS feature_flags (
  organization_id varchar(36) NOT NULL,
  flag_key varchar(160) NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, flag_key)
);

-- +goose Down
DROP TABLE IF EXISTS feature_flags;
DROP TABLE IF EXISTS consumed_messages;
DROP TABLE IF EXISTS reliable_messages;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS password_history;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
