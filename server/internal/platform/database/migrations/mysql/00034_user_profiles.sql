-- +goose Up
CREATE TABLE IF NOT EXISTS user_profiles (
  user_id varchar(36) PRIMARY KEY,
  real_name varchar(200) NOT NULL DEFAULT '',
  gender varchar(20) NOT NULL DEFAULT 'UNSPECIFIED',
  phone_country_code varchar(8) NOT NULL DEFAULT '+86',
  phone varchar(32) NOT NULL DEFAULT '',
  phone_verified_at timestamp NULL,
  email_verified_at timestamp NULL,
  avatar_url text NOT NULL,
  profile_version bigint NOT NULL DEFAULT 1,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT user_profiles_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT user_profiles_gender_ck CHECK (gender IN ('UNSPECIFIED','MALE','FEMALE')),
  CONSTRAINT user_profiles_version_ck CHECK (profile_version > 0),
  KEY user_profiles_phone_idx (phone)
);

-- +goose Down
DROP TABLE IF EXISTS user_profiles;
