-- +goose Up
CREATE TABLE IF NOT EXISTS portal_categories (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  category_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(500) NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'ACTIVE',
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, category_key)
);
CREATE INDEX IF NOT EXISTS idx_portal_categories_org_status ON portal_categories(organization_id, status, sort_order, category_key);

CREATE TABLE IF NOT EXISTS portal_tags (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  tag_key varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, tag_key)
);
CREATE INDEX IF NOT EXISTS idx_portal_tags_org_sort ON portal_tags(organization_id, sort_order, tag_key);

CREATE TABLE IF NOT EXISTS portal_applications (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  code varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  description varchar(1000) NOT NULL DEFAULT '',
  icon varchar(512) NOT NULL DEFAULT '',
  category_id varchar(36) NULL REFERENCES portal_categories(id) ON DELETE SET NULL,
  home_url varchar(2048) NOT NULL DEFAULT '',
  launch_url varchar(2048) NOT NULL DEFAULT '',
  launch_type varchar(30) NOT NULL DEFAULT 'URL',
  status varchar(20) NOT NULL DEFAULT 'ENABLED',
  sort_order integer NOT NULL DEFAULT 0,
  featured boolean NOT NULL DEFAULT false,
  created_by varchar(36) NOT NULL DEFAULT '',
  updated_by varchar(36) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, code)
);
CREATE INDEX IF NOT EXISTS idx_portal_apps_org_status ON portal_applications(organization_id, status, sort_order, name);
CREATE INDEX IF NOT EXISTS idx_portal_apps_category ON portal_applications(organization_id, category_id);

CREATE TABLE IF NOT EXISTS portal_application_tags (
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  tag_id varchar(36) NOT NULL REFERENCES portal_tags(id) ON DELETE CASCADE,
  PRIMARY KEY (application_id, tag_id)
);

CREATE TABLE IF NOT EXISTS portal_access_policies (
  id varchar(36) PRIMARY KEY,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  policy_type varchar(30) NOT NULL,
  value varchar(200) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_portal_policies_app ON portal_access_policies(application_id, policy_type, value);

CREATE TABLE IF NOT EXISTS portal_favorites (
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, user_id, application_id)
);

CREATE TABLE IF NOT EXISTS portal_visits (
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  application_id varchar(36) NOT NULL REFERENCES portal_applications(id) ON DELETE CASCADE,
  visit_count bigint NOT NULL DEFAULT 1,
  last_visited_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, user_id, application_id)
);
CREATE INDEX IF NOT EXISTS idx_portal_visits_recent ON portal_visits(organization_id, user_id, last_visited_at DESC);

-- +goose Down
DROP TABLE IF EXISTS portal_visits;
DROP TABLE IF EXISTS portal_favorites;
DROP TABLE IF EXISTS portal_access_policies;
DROP TABLE IF EXISTS portal_application_tags;
DROP TABLE IF EXISTS portal_applications;
DROP TABLE IF EXISTS portal_tags;
DROP TABLE IF EXISTS portal_categories;
