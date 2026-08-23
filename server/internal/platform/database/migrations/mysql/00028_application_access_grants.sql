-- +goose Up
CREATE TABLE IF NOT EXISTS application_access_grants (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  application_id varchar(36) NOT NULL,
  subject_type varchar(32) NOT NULL,
  subject_id varchar(100) NOT NULL DEFAULT '',
  include_descendants boolean NOT NULL DEFAULT false,
  effect varchar(16) NOT NULL,
  valid_from datetime(6) NULL,
  valid_until datetime(6) NULL,
  status varchar(20) NOT NULL,
  reason varchar(500) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  created_by varchar(36) NOT NULL,
  updated_by varchar(36) NOT NULL,
  created_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_access_grants_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_access_grants_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE,
  CONSTRAINT fk_access_grants_creator FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT fk_access_grants_updater FOREIGN KEY (updated_by) REFERENCES users(id),
  KEY idx_access_grants_app_status (organization_id, application_id, status),
  KEY idx_access_grants_subject (organization_id, subject_type, subject_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS application_access_grant_roles (
  grant_id varchar(36) NOT NULL,
  role_key varchar(100) NOT NULL,
  PRIMARY KEY(grant_id, role_key),
  CONSTRAINT fk_access_grant_roles_grant FOREIGN KEY (grant_id) REFERENCES application_access_grants(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_application_entitlement_sources (
  user_id varchar(36) NOT NULL,
  application_id varchar(36) NOT NULL,
  grant_id varchar(36) NOT NULL,
  created_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(user_id, application_id, grant_id),
  CONSTRAINT fk_entitlement_sources_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_entitlement_sources_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE,
  CONSTRAINT fk_entitlement_sources_grant FOREIGN KEY (grant_id) REFERENCES application_access_grants(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO application_access_grants(id,organization_id,application_id,subject_type,subject_id,include_descendants,effect,status,reason,version,created_by,updated_by,created_at,updated_at)
SELECT LEFT(MD5(CONCAT('policy:',p.id)),32),a.organization_id,p.application_id,
  CASE p.policy_type WHEN 'EVERYONE' THEN 'EVERYONE' WHEN 'ORGANIZATION' THEN 'EVERYONE' WHEN 'ROLE' THEN 'PLATFORM_ROLE' WHEN 'GROUP' THEN 'USER_GROUP' ELSE 'USER' END,
  CASE WHEN p.policy_type IN ('EVERYONE','ORGANIZATION') THEN '' WHEN p.policy_type='GROUP' THEN COALESCE(ug.id,'') ELSE p.value END,
  false,'ALLOW','ACTIVE','历史访问策略迁移',1,a.created_by,a.created_by,p.created_at,p.updated_at
FROM portal_access_policies p
JOIN portal_applications a ON a.id=p.application_id
LEFT JOIN user_groups ug ON p.policy_type='GROUP' AND ug.organization_id=a.organization_id AND ug.group_key=p.value
WHERE p.policy_type<>'GROUP' OR ug.id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS user_application_entitlement_sources;
DROP TABLE IF EXISTS application_access_grant_roles;
DROP TABLE IF EXISTS application_access_grants;
