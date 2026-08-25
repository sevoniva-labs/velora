-- +goose Up
CREATE TABLE IF NOT EXISTS user_profiles (
  user_id varchar(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  real_name varchar(200) NOT NULL DEFAULT '',
  gender varchar(20) NOT NULL DEFAULT 'UNSPECIFIED' CHECK (gender IN ('UNSPECIFIED','MALE','FEMALE')),
  phone_country_code varchar(8) NOT NULL DEFAULT '+86',
  phone varchar(32) NOT NULL DEFAULT '',
  phone_verified_at timestamptz NULL,
  email_verified_at timestamptz NULL,
  avatar_url text NOT NULL DEFAULT '',
  profile_version bigint NOT NULL DEFAULT 1 CHECK (profile_version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS user_profiles_phone_idx ON user_profiles(phone) WHERE phone <> '';

-- +goose Down
DROP TABLE IF EXISTS user_profiles;
