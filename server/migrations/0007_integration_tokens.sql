-- Velora 集成令牌（service account，Phase D3）
--
-- 供外部系统以 Bearer token 调用集成端点（如推送待办），与用户会话解耦。
-- token 明文仅创建时返回一次，库中存 SHA-256 哈希；支持吊销与过期。

CREATE TABLE IF NOT EXISTS integration_tokens (
    id           BIGSERIAL PRIMARY KEY,
    name         VARCHAR(128) NOT NULL,               -- 令牌名称（标识归属系统，如"工单系统"）
    token_hash   VARCHAR(64)  NOT NULL UNIQUE,        -- SHA-256(token) 十六进制
    scopes       TEXT         NOT NULL DEFAULT '',    -- 权限 scope（逗号分隔，如 todo:write）
    created_by   VARCHAR(128) NOT NULL DEFAULT '',    -- 创建者（管理员用户名）
    expires_at   TIMESTAMPTZ  NULL,                   -- 过期时间（NULL 为永不过期）
    revoked      BOOLEAN      NOT NULL DEFAULT FALSE,
    last_used_at TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_integration_tokens_name ON integration_tokens(name);
CREATE INDEX IF NOT EXISTS idx_integration_tokens_revoked ON integration_tokens(revoked) WHERE revoked = FALSE;
