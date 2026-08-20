-- +goose Up
ALTER TABLE sessions ADD COLUMN authentication_level varchar(20) NOT NULL DEFAULT 'PASSWORD';
ALTER TABLE sessions ADD COLUMN mfa_verified_at timestamp(6) NULL;

-- +goose Down
ALTER TABLE sessions DROP COLUMN mfa_verified_at;
ALTER TABLE sessions DROP COLUMN authentication_level;
