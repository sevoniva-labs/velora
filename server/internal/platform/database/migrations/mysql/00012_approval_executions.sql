-- +goose Up
CREATE TABLE IF NOT EXISTS approval_executions (
  id varchar(36) PRIMARY KEY,
  request_id varchar(36) NOT NULL,
  executed_by varchar(36) NOT NULL,
  request_digest varchar(128) NOT NULL,
  executed_at timestamp(6) NOT NULL,
  UNIQUE KEY uk_approval_executions_request (request_id),
  CONSTRAINT fk_approval_executions_request FOREIGN KEY (request_id) REFERENCES approval_requests(id) ON DELETE RESTRICT,
  CONSTRAINT fk_approval_executions_actor FOREIGN KEY (executed_by) REFERENCES users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS approval_executions;
