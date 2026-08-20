-- +goose Up
CREATE TABLE IF NOT EXISTS approval_requests (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  request_type varchar(100) NOT NULL,
  action varchar(160) NOT NULL,
  resource varchar(160) NOT NULL,
  resource_id varchar(160) NOT NULL DEFAULT '',
  summary varchar(500) NOT NULL,
  request_digest varchar(128) NOT NULL,
  applicant_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  approval_mode varchar(20) NOT NULL,
  required_approvals integer NOT NULL,
  status varchar(20) NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_approval_requests_org_status ON approval_requests(organization_id, status, created_at DESC);
CREATE TABLE IF NOT EXISTS approval_tasks (
  id varchar(36) PRIMARY KEY,
  request_id varchar(36) NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
  assignee_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  status varchar(20) NOT NULL,
  decision varchar(20) NOT NULL DEFAULT '',
  comment varchar(1000) NOT NULL DEFAULT '',
  transferred_from varchar(36) NOT NULL DEFAULT '',
  decided_at timestamptz NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (request_id, assignee_id)
);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_assignee_status ON approval_tasks(assignee_id, status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS approval_tasks;
DROP TABLE IF EXISTS approval_requests;
