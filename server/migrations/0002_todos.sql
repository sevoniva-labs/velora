-- Velora 待办中心。
-- 设计目标：外部系统（OA / 运维工单 / 审批流等）可通过 API 将待办推送进来，
-- 以 (source_system, source_id, user_id) 为幂等键，重复推送只更新不产生重复待办。

CREATE TABLE todos (
    id            BIGSERIAL PRIMARY KEY,
    user_id       VARCHAR(128) NOT NULL,                -- 归属用户（Casdoor user id）
    title         VARCHAR(256) NOT NULL,
    source_system VARCHAR(64)  NOT NULL DEFAULT '',     -- 来源系统标识，如 oa / ops
    source_label  VARCHAR(128) NOT NULL DEFAULT '',     -- 来源展示名，如 OA 审批
    source_id     VARCHAR(128) NOT NULL DEFAULT '',     -- 来源系统单据 id（幂等键组成部分）
    priority      VARCHAR(16)  NOT NULL DEFAULT 'mid',  -- urgent | high | mid | low
    url           VARCHAR(512) NOT NULL DEFAULT '',     -- 点击跳转地址（来源系统单据页）
    due_at        TIMESTAMPTZ,                          -- 到期时间，可空
    status        VARCHAR(16)  NOT NULL DEFAULT 'open', -- open | done
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_todos_source ON todos (source_system, source_id, user_id);

CREATE INDEX idx_todos_user_status ON todos (user_id, status, due_at);
