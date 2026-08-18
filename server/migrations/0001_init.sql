-- Velora 第一阶段核心表结构。
-- 约定：sso_type 枚举 URL | OIDC | SAML | CAS | FORWARD_AUTH（第一阶段仅实现 URL / OIDC）。

CREATE TABLE application_categories (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64)  NOT NULL UNIQUE,
    name        VARCHAR(128) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    sort        INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE application_tags (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64)  NOT NULL UNIQUE,
    name        VARCHAR(128) NOT NULL,
    sort        INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE applications (
    id                      BIGSERIAL PRIMARY KEY,
    code                    VARCHAR(64)   NOT NULL UNIQUE,
    name                    VARCHAR(128)  NOT NULL,
    description             TEXT          NOT NULL DEFAULT '',
    keywords                TEXT          NOT NULL DEFAULT '',
    icon                    VARCHAR(512)  NOT NULL DEFAULT '',
    category_id             BIGINT        REFERENCES application_categories(id) ON DELETE SET NULL,
    home_url                VARCHAR(2048) NOT NULL DEFAULT '',
    launch_url              VARCHAR(2048) NOT NULL DEFAULT '',
    sso_type                VARCHAR(32)   NOT NULL DEFAULT 'URL',
    casdoor_application_name VARCHAR(128) NOT NULL DEFAULT '',
    casdoor_client_id       VARCHAR(128)  NOT NULL DEFAULT '',
    owner                   VARCHAR(128)  NOT NULL DEFAULT '',
    department              VARCHAR(128)  NOT NULL DEFAULT '',
    status                  VARCHAR(16)   NOT NULL DEFAULT 'ENABLED',
    sort                    INT           NOT NULL DEFAULT 0,
    is_featured             BOOLEAN       NOT NULL DEFAULT FALSE,
    health_check_enabled    BOOLEAN       NOT NULL DEFAULT FALSE,
    health_check_url        VARCHAR(2048) NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT now(),
    created_by              VARCHAR(128)  NOT NULL DEFAULT '',
    updated_by              VARCHAR(128)  NOT NULL DEFAULT ''
);

CREATE INDEX idx_applications_category  ON applications(category_id);
CREATE INDEX idx_applications_status    ON applications(status);
CREATE INDEX idx_applications_name      ON applications(name);
CREATE INDEX idx_applications_featured  ON applications(is_featured);

CREATE TABLE application_tag_relations (
    application_id BIGINT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    tag_id         BIGINT NOT NULL REFERENCES application_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (application_id, tag_id)
);

CREATE TABLE application_access_policies (
    id             BIGSERIAL PRIMARY KEY,
    application_id BIGINT       NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    policy_type    VARCHAR(32)  NOT NULL,
    value          VARCHAR(256) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_access_policies_app ON application_access_policies(application_id);

CREATE TABLE application_favorites (
    user_id        VARCHAR(128) NOT NULL,
    application_id BIGINT       NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, application_id)
);

CREATE TABLE application_visits (
    user_id         VARCHAR(128) NOT NULL,
    application_id  BIGINT       NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    visit_count     BIGINT       NOT NULL DEFAULT 1,
    last_visited_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, application_id)
);

CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    operator    VARCHAR(128) NOT NULL DEFAULT '',
    action      VARCHAR(64)  NOT NULL,
    resource    VARCHAR(64)  NOT NULL DEFAULT '',
    resource_id VARCHAR(128) NOT NULL DEFAULT '',
    ip          VARCHAR(64)  NOT NULL DEFAULT '',
    user_agent  VARCHAR(512) NOT NULL DEFAULT '',
    request_id  VARCHAR(64)  NOT NULL DEFAULT '',
    detail      TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_created  ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_action   ON audit_logs(action);
CREATE INDEX idx_audit_operator ON audit_logs(operator);

CREATE TABLE portal_settings (
    key        VARCHAR(128) PRIMARY KEY,
    value      TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
