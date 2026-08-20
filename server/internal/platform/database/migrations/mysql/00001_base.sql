-- +goose Up
CREATE TABLE IF NOT EXISTS organizations (
  id varchar(36) PRIMARY KEY,
  org_key varchar(100) NOT NULL UNIQUE,
  name varchar(200) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS users (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  login_name varchar(120) NOT NULL,
  display_name varchar(200) NOT NULL DEFAULT '',
  password_hash text NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  must_change_password boolean NOT NULL DEFAULT true,
  failed_login_count int NOT NULL DEFAULT 0,
  locked_until timestamp(6) NULL,
  password_changed_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_users_org_login (organization_id, login_name),
  KEY idx_users_org_status (organization_id, status),
  CONSTRAINT fk_users_org FOREIGN KEY (organization_id) REFERENCES organizations(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS roles (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  role_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_roles_org_key (organization_id, role_key),
  CONSTRAINT fk_roles_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS permissions (
  id varchar(36) PRIMARY KEY,
  permission_key varchar(160) NOT NULL UNIQUE,
  name varchar(200) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS user_roles (
  user_id varchar(36) NOT NULL,
  role_id varchar(36) NOT NULL,
  PRIMARY KEY (user_id, role_id),
  CONSTRAINT fk_ur_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_ur_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS role_permissions (
  role_id varchar(36) NOT NULL,
  permission_id varchar(36) NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  CONSTRAINT fk_rp_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_rp_perm FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS password_history (
  id varchar(36) PRIMARY KEY,
  user_id varchar(36) NOT NULL,
  password_hash text NOT NULL,
  changed_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_password_history_user_time (user_id, changed_at),
  CONSTRAINT fk_ph_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sessions (
  id varchar(36) PRIMARY KEY,
  user_id varchar(36) NOT NULL,
  token_hash varchar(64) NOT NULL UNIQUE,
  expires_at timestamp(6) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  last_seen_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  client_ip varchar(64) NOT NULL DEFAULT '',
  user_agent varchar(512) NOT NULL DEFAULT '',
  KEY idx_sessions_expires_at (expires_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS api_tokens (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  name varchar(120) NOT NULL,
  token_prefix varchar(24) NOT NULL,
  token_hash varchar(64) NOT NULL UNIQUE,
  scopes_json text NOT NULL,
  expires_at timestamp(6) NULL,
  last_used_at timestamp(6) NULL,
  revoked_at timestamp(6) NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_api_tokens_user (user_id, created_at),
  KEY idx_api_tokens_expiry (expires_at),
  CONSTRAINT fk_api_tokens_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_api_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_logs (
  id varchar(36) PRIMARY KEY,
  occurred_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  request_id varchar(80) NOT NULL DEFAULT '',
  organization_id varchar(36) NULL,
  actor_id varchar(36) NULL,
  actor_name varchar(200) NOT NULL DEFAULT '',
  action varchar(200) NOT NULL,
  resource_type varchar(100) NOT NULL DEFAULT '',
  resource_id varchar(100) NOT NULL DEFAULT '',
  result varchar(20) NOT NULL DEFAULT 'SUCCESS',
  client_ip varchar(64) NOT NULL DEFAULT '',
  details_json text NOT NULL,
  KEY idx_audit_logs_occurred_at (occurred_at),
  KEY idx_audit_logs_org_time (organization_id, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS idempotency_records (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  scope varchar(120) NOT NULL,
  idem_key varchar(160) NOT NULL,
  request_hash varchar(64) NOT NULL,
  state varchar(20) NOT NULL DEFAULT 'PROCESSING',
  response_status int NULL,
  response_body longblob NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  expires_at timestamp(6) NOT NULL,
  UNIQUE KEY uk_idem_org_scope_key (organization_id, scope, idem_key),
  KEY idx_idem_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS reliable_messages (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NULL,
  topic varchar(200) NOT NULL,
  event_key varchar(200) NOT NULL DEFAULT '',
  event_type varchar(160) NOT NULL,
  ordering_key varchar(200) NOT NULL DEFAULT '',
  tag varchar(128) NOT NULL DEFAULT '',
  deliver_at timestamp(6) NULL,
  payload_json longtext NOT NULL,
  headers_json text NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'PENDING',
  attempts int NOT NULL DEFAULT 0,
  next_attempt_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  published_at timestamp(6) NULL,
  last_error varchar(1000) NOT NULL DEFAULT '',
  KEY idx_reliable_messages_pending (status, next_attempt_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS consumed_messages (
  consumer_group varchar(160) NOT NULL,
  event_id varchar(200) NOT NULL,
  organization_id varchar(36) NULL,
  topic varchar(200) NOT NULL,
  event_type varchar(160) NOT NULL,
  body_hash char(64) NOT NULL,
  provider_message_id varchar(200) NOT NULL DEFAULT '',
  consumed_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (consumer_group, event_id),
  KEY idx_consumed_messages_at (consumed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS feature_flags (
  organization_id varchar(36) NOT NULL,
  flag_key varchar(160) NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, flag_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

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
