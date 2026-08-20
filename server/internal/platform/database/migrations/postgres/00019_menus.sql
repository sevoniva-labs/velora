-- +goose Up
CREATE TABLE IF NOT EXISTS menus (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    menu_key varchar(160) NOT NULL,
    parent_key varchar(160) NOT NULL DEFAULT '',
    name varchar(200) NOT NULL,
    route varchar(300) NOT NULL,
    icon varchar(100) NOT NULL DEFAULT '',
    permission_key varchar(160) NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, menu_key)
);
CREATE INDEX IF NOT EXISTS idx_menus_org_parent_order ON menus(organization_id, parent_key, sort_order, menu_key);

-- +goose Down
DROP TABLE IF EXISTS menus;
