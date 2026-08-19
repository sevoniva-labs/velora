-- Velora OIDC Provider（Phase B）：让 Velora 成为企业 SSO 登录终点。
-- 设计原则：
--   - code / refresh token 一律只存哈希（SHA-256），数据库泄露不可直接使用；
--   - 签名密钥对独立表（oidc_keys），支持 90 天轮换 + 30 天宽限期；
--   - 第三方应用 client_secret 只存哈希（bcrypt），页面仅创建时展示一次。
--
-- 同时包含邮件同步多实例互斥表（mail_sync_lease，Phase D4/Phase A3 配套）。

-- OIDC 客户端（第三方应用接入 Velora SSO）
CREATE TABLE oidc_clients (
    id                 BIGSERIAL PRIMARY KEY,
    application_id     BIGINT REFERENCES applications(id) ON DELETE CASCADE,
    client_id          VARCHAR(128) NOT NULL UNIQUE,
    client_secret_hash VARCHAR(256) NOT NULL,           -- bcrypt（应用方保存原文）
    redirect_uris      TEXT NOT NULL DEFAULT '[]',      -- JSON 数组，白名单
    grant_types        TEXT NOT NULL DEFAULT '["authorization_code"]',  -- JSON 数组
    scopes             TEXT NOT NULL DEFAULT '["openid","profile","email"]',
    token_endpoint_auth_method VARCHAR(32) NOT NULL DEFAULT 'client_secret_post',
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oidc_clients_application ON oidc_clients (application_id);

-- 一次性授权码（10 分钟有效，单次使用）
CREATE TABLE oidc_auth_codes (
    id           BIGSERIAL PRIMARY KEY,
    code_hash    VARCHAR(64) NOT NULL UNIQUE,           -- SHA-256 hex
    client_id    VARCHAR(128) NOT NULL,
    user_id      VARCHAR(128) NOT NULL,
    redirect_uri VARCHAR(512) NOT NULL,
    scope        TEXT NOT NULL DEFAULT '',
    code_challenge     VARCHAR(256) NOT NULL DEFAULT '',
    code_challenge_method VARCHAR(16) NOT NULL DEFAULT 'S256',
    nonce        VARCHAR(128) NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    used         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oidc_codes_client ON oidc_auth_codes (client_id, expires_at);

-- 令牌（access_token 用 JWT 无状态短时；refresh_token 落库可吊销）
CREATE TABLE oidc_tokens (
    id           BIGSERIAL PRIMARY KEY,
    client_id    VARCHAR(128) NOT NULL,
    user_id      VARCHAR(128) NOT NULL,
    access_jti   VARCHAR(64) NOT NULL UNIQUE,           -- JWT jti（SHA-256 hex，吊销索引）
    refresh_hash VARCHAR(64) NOT NULL UNIQUE,           -- SHA-256 hex
    scope        TEXT NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oidc_tokens_user ON oidc_tokens (user_id);
CREATE INDEX idx_oidc_tokens_revoked ON oidc_tokens (revoked_at);

-- 签名密钥对（RS256；轮换时新增行，旧 key 保留宽限期）
CREATE TABLE oidc_keys (
    kid         VARCHAR(64) PRIMARY KEY,
    alg         VARCHAR(16) NOT NULL DEFAULT 'RS256',
    public_pem  TEXT NOT NULL,
    private_pem TEXT NOT NULL,                          -- 建议加密存储（AES-GCM，密钥来自 KMS/env）
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    not_before  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ
);

-- 邮件同步多实例互斥（DB lease，Phase A3/D4）
CREATE TABLE mail_sync_lease (
    name        VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
