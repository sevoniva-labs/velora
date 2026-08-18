-- Velora 邮件模块 + 待办类型。
-- 设计原则：Mail 是独立领域，与 Todo 解耦；Todo 通过 source_system='mail' + source_id=mail_messages.id 引用邮件。
-- Provider 仅决定默认主机配置（Profile），业务逻辑不感知具体邮箱厂商。

-- 待办类型（Tab 维度）：mail | approval | devops | ops | project | hr | other
ALTER TABLE todos ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT 'other';

CREATE INDEX idx_todos_user_kind ON todos (user_id, status, kind);

-- 邮件账号：用户绑定自己的企业邮箱；凭证 AES-256-GCM 加密存储（密钥来自环境变量）。
CREATE TABLE mail_accounts (
    id            BIGSERIAL PRIMARY KEY,
    user_id       VARCHAR(128) NOT NULL,
    provider      VARCHAR(32)  NOT NULL DEFAULT 'imap',      -- imap | aliyun | tencent | custom（Profile 仅提供默认值）
    email         VARCHAR(256) NOT NULL,
    display_name  VARCHAR(128) NOT NULL DEFAULT '',
    auth_type     VARCHAR(16)  NOT NULL DEFAULT 'password',  -- password（授权码）| oauth（预留）
    credential    TEXT         NOT NULL DEFAULT '',          -- AES-256-GCM 密文（base64）
    imap_host     VARCHAR(256) NOT NULL,
    imap_port     INT          NOT NULL DEFAULT 993,
    smtp_host     VARCHAR(256) NOT NULL DEFAULT '',
    smtp_port     INT          NOT NULL DEFAULT 465,
    status        VARCHAR(16)  NOT NULL DEFAULT 'active',    -- active | error | disabled
    sync_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    unread_count  INT          NOT NULL DEFAULT 0,           -- 同步后缓存，供待办 Tab 角标
    last_sync_at  TIMESTAMPTZ,
    last_error    TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_mail_accounts_user_email ON mail_accounts (user_id, email);

-- 邮件消息：同步只落元数据 + 摘要；正文按需拉取后缓存（body_text/body_html 可能为空）。
CREATE TABLE mail_messages (
    id             BIGSERIAL PRIMARY KEY,
    account_id     BIGINT       NOT NULL REFERENCES mail_accounts(id) ON DELETE CASCADE,
    user_id        VARCHAR(128) NOT NULL,                    -- 冗余归属用户，便于按用户过滤
    folder         VARCHAR(64)  NOT NULL DEFAULT 'INBOX',
    uid            BIGINT       NOT NULL,                    -- IMAP UID（folder 内唯一）
    message_id     VARCHAR(256) NOT NULL DEFAULT '',         -- RFC822 Message-ID（可能缺失，不作唯一依据）
    subject        VARCHAR(512) NOT NULL DEFAULT '',
    from_address   VARCHAR(256) NOT NULL DEFAULT '',
    from_name      VARCHAR(256) NOT NULL DEFAULT '',
    to_addresses   TEXT         NOT NULL DEFAULT '',
    received_at    TIMESTAMPTZ,
    is_read        BOOLEAN      NOT NULL DEFAULT FALSE,
    is_starred     BOOLEAN      NOT NULL DEFAULT FALSE,
    has_attachment BOOLEAN      NOT NULL DEFAULT FALSE,
    snippet        VARCHAR(512) NOT NULL DEFAULT '',
    body_text      TEXT         NOT NULL DEFAULT '',
    body_html      TEXT         NOT NULL DEFAULT '',
    size           INT          NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_mail_messages_uid ON mail_messages (account_id, folder, uid);
CREATE INDEX idx_mail_messages_user ON mail_messages (user_id, folder, received_at DESC);

-- 同步游标：UIDVALIDITY / last_uid，支撑增量同步与断线恢复。
CREATE TABLE mail_sync_state (
    account_id    BIGINT      NOT NULL REFERENCES mail_accounts(id) ON DELETE CASCADE,
    folder        VARCHAR(64) NOT NULL,
    uid_validity  BIGINT      NOT NULL DEFAULT 0,
    last_uid      BIGINT      NOT NULL DEFAULT 0,
    last_sync_at  TIMESTAMPTZ,
    sync_status   VARCHAR(16) NOT NULL DEFAULT 'ok',         -- ok | error
    error         TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (account_id, folder)
);
