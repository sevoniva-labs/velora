-- +goose Up
ALTER TABLE approval_requests ADD COLUMN payload_json text NULL;
UPDATE approval_requests SET payload_json='{}' WHERE payload_json IS NULL;
ALTER TABLE approval_requests MODIFY COLUMN payload_json text NOT NULL;

-- +goose Down
ALTER TABLE approval_requests DROP COLUMN payload_json;
