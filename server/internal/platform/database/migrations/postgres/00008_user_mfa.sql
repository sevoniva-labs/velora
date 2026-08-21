-- +goose Up
CREATE TABLE IF NOT EXISTS user_mfa_factors (
  user_id varchar(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  status varchar(20) NOT NULL,
  secret_ciphertext text NOT NULL,
  key_version varchar(50) NOT NULL,
  pending_expires_at timestamptz NOT NULL,
  confirmed_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS mfa_recovery_codes (
  id varchar(36) PRIMARY KEY,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash varchar(128) NOT NULL,
  used_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, code_hash)
);
CREATE INDEX IF NOT EXISTS idx_mfa_recovery_codes_user_unused ON mfa_recovery_codes(user_id, used_at);

-- +goose Down
DROP TABLE IF EXISTS mfa_recovery_codes;
DROP TABLE IF EXISTS user_mfa_factors;
