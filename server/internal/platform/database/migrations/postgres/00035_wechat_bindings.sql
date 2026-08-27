-- +goose Up
CREATE TABLE IF NOT EXISTS user_wechat_bindings (
  user_id varchar(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  provider varchar(100) NOT NULL,
  bound_at timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz NULL,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE INDEX IF NOT EXISTS user_wechat_bindings_provider_idx ON user_wechat_bindings(provider);

-- +goose Down
DROP TABLE IF EXISTS user_wechat_bindings;
