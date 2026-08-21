-- +goose Up
CREATE TABLE IF NOT EXISTS approval_executions (
  id varchar(36) PRIMARY KEY,
  request_id varchar(36) NOT NULL UNIQUE REFERENCES approval_requests(id) ON DELETE RESTRICT,
  executed_by varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  request_digest varchar(128) NOT NULL,
  executed_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS approval_executions;
