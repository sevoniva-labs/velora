-- +goose Up
ALTER TABLE portal_application_identity_bindings ADD COLUMN environments_json longtext NULL;
UPDATE portal_application_identity_bindings SET environments_json='[]' WHERE environments_json IS NULL OR environments_json='';
ALTER TABLE portal_application_identity_bindings MODIFY COLUMN environments_json longtext NOT NULL;

-- +goose Down
ALTER TABLE portal_application_identity_bindings DROP COLUMN environments_json;
