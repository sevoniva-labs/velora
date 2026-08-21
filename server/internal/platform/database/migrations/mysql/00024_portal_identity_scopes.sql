-- +goose Up
ALTER TABLE portal_application_identity_bindings
  ADD COLUMN scopes_json text NOT NULL;
UPDATE portal_application_identity_bindings SET scopes_json='["openid","profile","email"]' WHERE scopes_json='';

-- +goose Down
ALTER TABLE portal_application_identity_bindings DROP COLUMN scopes_json;
