-- +goose Up
CREATE TABLE IF NOT EXISTS user_mfa_factors (
  user_id varchar(36) PRIMARY KEY,
  status varchar(20) NOT NULL,
  secret_ciphertext text NOT NULL,
  key_version varchar(50) NOT NULL,
  pending_expires_at timestamp(6) NOT NULL,
  confirmed_at timestamp(6) NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_user_mfa_factor_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mfa_recovery_codes (
  id varchar(36) PRIMARY KEY,
  user_id varchar(36) NOT NULL,
  code_hash varchar(128) NOT NULL,
  used_at timestamp(6) NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_mfa_recovery_user_hash (user_id, code_hash),
  KEY idx_mfa_recovery_codes_user_unused (user_id, used_at),
  CONSTRAINT fk_mfa_recovery_code_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS mfa_recovery_codes;
DROP TABLE IF EXISTS user_mfa_factors;
