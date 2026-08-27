-- +goose Up
CREATE TABLE IF NOT EXISTS user_wechat_bindings (
  user_id varchar(36) NOT NULL PRIMARY KEY,
  provider varchar(100) NOT NULL,
  bound_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  last_login_at datetime(6) NULL,
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT fk_user_wechat_bindings_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  INDEX user_wechat_bindings_provider_idx (provider)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS user_wechat_bindings;
