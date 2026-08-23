-- +goose Up
ALTER TABLE portal_application_provisioning_targets
  ALTER COLUMN secret_ref TYPE text;

-- +goose Down
-- Intentionally irreversible: encrypted envelope values may exceed the legacy
-- varchar(512) limit. Narrowing this column could make an application rollback
-- fail after production data has been written.
SELECT 1;
