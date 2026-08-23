-- +goose Up
ALTER TABLE roles ADD COLUMN description varchar(500) NOT NULL DEFAULT '';
ALTER TABLE roles ADD COLUMN status varchar(20) NOT NULL DEFAULT 'ACTIVE';
CREATE INDEX roles_org_status ON roles(organization_id,status,role_key);

-- +goose Down
DROP INDEX roles_org_status ON roles;
ALTER TABLE roles DROP COLUMN status;
ALTER TABLE roles DROP COLUMN description;
