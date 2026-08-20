-- +goose Up
CREATE TABLE IF NOT EXISTS user_groups (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  group_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, group_key)
);

CREATE TABLE IF NOT EXISTS user_group_members (
  group_id varchar(36) NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_user_group_members_user ON user_group_members(user_id, group_id);

CREATE TABLE IF NOT EXISTS user_group_roles (
  group_id varchar(36) NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  role_id varchar(36) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, role_id)
);

-- +goose Down
DROP TABLE IF EXISTS user_group_roles;
DROP TABLE IF EXISTS user_group_members;
DROP TABLE IF EXISTS user_groups;
