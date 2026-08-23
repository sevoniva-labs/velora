-- +goose Up
ALTER TABLE portal_applications
  ADD COLUMN owner_user_id varchar(36) NULL,
  ADD COLUMN owner_department_id varchar(36) NULL,
  ADD CONSTRAINT fk_portal_app_owner_user FOREIGN KEY (owner_user_id) REFERENCES users(id),
  ADD CONSTRAINT fk_portal_app_owner_department FOREIGN KEY (owner_department_id) REFERENCES departments(id),
  ADD KEY idx_portal_app_owner_user (organization_id, owner_user_id),
  ADD KEY idx_portal_app_owner_department (organization_id, owner_department_id);

-- +goose Down
ALTER TABLE portal_applications
  DROP FOREIGN KEY fk_portal_app_owner_department,
  DROP FOREIGN KEY fk_portal_app_owner_user,
  DROP INDEX idx_portal_app_owner_department,
  DROP INDEX idx_portal_app_owner_user,
  DROP COLUMN owner_department_id,
  DROP COLUMN owner_user_id;
