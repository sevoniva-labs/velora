-- +goose Up
ALTER TABLE portal_application_identity_bindings ADD COLUMN IF NOT EXISTS environments_json text NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE portal_application_identity_bindings DROP COLUMN IF EXISTS environments_json;
