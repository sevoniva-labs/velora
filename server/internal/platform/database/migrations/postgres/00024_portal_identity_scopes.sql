-- +goose Up
ALTER TABLE portal_application_identity_bindings
  ADD COLUMN IF NOT EXISTS scopes_json text NOT NULL DEFAULT '["openid","profile","email"]';

-- +goose Down
ALTER TABLE portal_application_identity_bindings DROP COLUMN IF EXISTS scopes_json;
