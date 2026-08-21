-- +goose Up
CREATE TABLE IF NOT EXISTS portal_categories (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  category_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  sort_order int NOT NULL DEFAULT 0,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_portal_categories_org_key (organization_id, category_key),
  KEY idx_portal_categories_org_status (organization_id, status, sort_order, category_key),
  CONSTRAINT fk_portal_categories_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_tags (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  tag_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  sort_order int NOT NULL DEFAULT 0,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_portal_tags_org_key (organization_id, tag_key),
  KEY idx_portal_tags_org_sort (organization_id, sort_order, tag_key),
  CONSTRAINT fk_portal_tags_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_applications (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  code varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(1000) NOT NULL DEFAULT '',
  icon varchar(512) NOT NULL DEFAULT '',
  category_id varchar(36) NULL,
  home_url varchar(2048) NOT NULL DEFAULT '',
  launch_url varchar(2048) NOT NULL DEFAULT '',
  launch_type varchar(30) NOT NULL DEFAULT 'URL',
  status varchar(20) NOT NULL DEFAULT 'ENABLED',
  sort_order int NOT NULL DEFAULT 0,
  featured boolean NOT NULL DEFAULT false,
  created_by varchar(36) NOT NULL DEFAULT '',
  updated_by varchar(36) NOT NULL DEFAULT '',
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_portal_apps_org_code (organization_id, code),
  KEY idx_portal_apps_org_status (organization_id, status, sort_order, name),
  KEY idx_portal_apps_category (organization_id, category_id),
  CONSTRAINT fk_portal_apps_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_apps_category FOREIGN KEY (category_id) REFERENCES portal_categories(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_application_tags (
  application_id varchar(36) NOT NULL,
  tag_id varchar(36) NOT NULL,
  PRIMARY KEY (application_id, tag_id),
  CONSTRAINT fk_portal_app_tags_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_app_tags_tag FOREIGN KEY (tag_id) REFERENCES portal_tags(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_access_policies (
  id varchar(36) PRIMARY KEY,
  application_id varchar(36) NOT NULL,
  policy_type varchar(30) NOT NULL,
  value varchar(200) NOT NULL DEFAULT '',
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_portal_policies_app (application_id, policy_type, value),
  CONSTRAINT fk_portal_policies_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_favorites (
  organization_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  application_id varchar(36) NOT NULL,
  created_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, user_id, application_id),
  CONSTRAINT fk_portal_favorites_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_favorites_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_favorites_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portal_visits (
  organization_id varchar(36) NOT NULL,
  user_id varchar(36) NOT NULL,
  application_id varchar(36) NOT NULL,
  visit_count bigint NOT NULL DEFAULT 1,
  last_visited_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, user_id, application_id),
  KEY idx_portal_visits_recent (organization_id, user_id, last_visited_at),
  CONSTRAINT fk_portal_visits_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_visits_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_visits_app FOREIGN KEY (application_id) REFERENCES portal_applications(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS portal_visits;
DROP TABLE IF EXISTS portal_favorites;
DROP TABLE IF EXISTS portal_access_policies;
DROP TABLE IF EXISTS portal_application_tags;
DROP TABLE IF EXISTS portal_applications;
DROP TABLE IF EXISTS portal_tags;
DROP TABLE IF EXISTS portal_categories;
