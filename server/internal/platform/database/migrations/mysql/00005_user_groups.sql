-- +goose Up
CREATE TABLE IF NOT EXISTS user_groups (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  group_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_user_groups_org_key (organization_id, group_key),
  KEY idx_user_groups_org_status (organization_id, status),
  CONSTRAINT fk_user_groups_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_group_members (
  group_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (group_id, user_id),
  KEY idx_user_group_members_user (user_id, group_id),
  CONSTRAINT fk_user_group_members_group FOREIGN KEY (group_id) REFERENCES user_groups(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_group_members_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_group_roles (
  group_id varchar(36) NOT NULL,
  role_id varchar(36) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (group_id, role_id),
  CONSTRAINT fk_user_group_roles_group FOREIGN KEY (group_id) REFERENCES user_groups(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_group_roles_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS user_group_roles;
DROP TABLE IF EXISTS user_group_members;
DROP TABLE IF EXISTS user_groups;
