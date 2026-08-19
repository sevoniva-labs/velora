-- Velora 服务端会话（Phase B6/C1）：从无状态 HMAC Cookie 升级为服务端可吊销会话。
--
-- 设计：
--   - 会话表以 session_id（HMAC 签名 payload 中的 sid）为主键索引；
--   - 支持吊销（强制下线）、设备标识（user-agent）、最后活跃时间；
--   - 用户快照（username/email/roles/groups）随会话落库：
--     服务端会话是 OIDC token 用户信息（preferred_username/roles）的来源，
--     不实时查 Casdoor（避免每次 token 签发打 Casdoor）。
--   - 兼容旧无状态会话：签名仍可解码（不做服务端校验），直到用户重新登录或
--     全部下线（改密/管理员强制下线时清除 sessions 表对应行）。

CREATE TABLE IF NOT EXISTS sessions (
    id            BIGSERIAL PRIMARY KEY,
    session_id    VARCHAR(128) NOT NULL UNIQUE,          -- HMAC payload 中的 sid
    user_id       VARCHAR(128) NOT NULL,
    username      VARCHAR(128) NOT NULL DEFAULT '',
    display_name  VARCHAR(256) NOT NULL DEFAULT '',
    email         VARCHAR(256) NOT NULL DEFAULT '',
    avatar        VARCHAR(512) NOT NULL DEFAULT '',
    organization  VARCHAR(128) NOT NULL DEFAULT '',
    roles         TEXT NOT NULL DEFAULT '[]',            -- JSON 数组
    groups        TEXT NOT NULL DEFAULT '[]',            -- JSON 数组
    user_agent    VARCHAR(512) NOT NULL DEFAULT '',
    ip            VARCHAR(64)  NOT NULL DEFAULT '',
    issued_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user ON sessions (user_id);
CREATE INDEX idx_sessions_revoked ON sessions (revoked_at) WHERE revoked_at IS NULL;
