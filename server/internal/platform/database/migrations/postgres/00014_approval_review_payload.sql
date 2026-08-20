-- +goose Up
ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS payload_json text NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE approval_requests DROP COLUMN IF EXISTS payload_json;
