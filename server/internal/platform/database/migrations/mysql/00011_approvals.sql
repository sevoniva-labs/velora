-- +goose Up
CREATE TABLE IF NOT EXISTS approval_requests (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  request_type varchar(100) NOT NULL,
  action varchar(160) NOT NULL,
  resource varchar(160) NOT NULL,
  resource_id varchar(160) NOT NULL DEFAULT '',
  summary varchar(500) NOT NULL,
  request_digest varchar(128) NOT NULL,
  applicant_id varchar(36) NOT NULL,
  approval_mode varchar(20) NOT NULL,
  required_approvals integer NOT NULL,
  status varchar(20) NOT NULL,
  expires_at timestamp(6) NOT NULL,
  created_at timestamp(6) NOT NULL,
  updated_at timestamp(6) NOT NULL,
  KEY idx_approval_requests_org_status (organization_id, status, created_at),
  CONSTRAINT fk_approval_requests_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_approval_requests_applicant FOREIGN KEY (applicant_id) REFERENCES users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS approval_tasks (
  id varchar(36) PRIMARY KEY,
  request_id varchar(36) NOT NULL,
  assignee_id varchar(36) NOT NULL,
  status varchar(20) NOT NULL,
  decision varchar(20) NOT NULL DEFAULT '',
  comment varchar(1000) NOT NULL DEFAULT '',
  transferred_from varchar(36) NOT NULL DEFAULT '',
  decided_at timestamp(6) NULL,
  created_at timestamp(6) NOT NULL,
  updated_at timestamp(6) NOT NULL,
  UNIQUE KEY uk_approval_tasks_request_assignee (request_id, assignee_id),
  KEY idx_approval_tasks_assignee_status (assignee_id, status, created_at),
  CONSTRAINT fk_approval_tasks_request FOREIGN KEY (request_id) REFERENCES approval_requests(id) ON DELETE CASCADE,
  CONSTRAINT fk_approval_tasks_assignee FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS approval_tasks;
DROP TABLE IF EXISTS approval_requests;
