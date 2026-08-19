-- Velora 性能优化（Phase D2）
--
-- 1) pg_trgm GIN 索引：加速应用搜索的 ILIKE '%kw%'（模糊匹配）。
-- 2) 权限过滤已下推 SQL（service.go），此迁移仅建索引。

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 应用搜索字段：name / code / description / keywords
CREATE INDEX IF NOT EXISTS idx_applications_name_trgm
    ON applications USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_applications_code_trgm
    ON applications USING GIN (code gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_applications_description_trgm
    ON applications USING GIN (description gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_applications_keywords_trgm
    ON applications USING GIN (keywords gin_trgm_ops);

-- 标签搜索（应用按标签名过滤）
CREATE INDEX IF NOT EXISTS idx_tags_name_trgm
    ON application_tags USING GIN (name gin_trgm_ops);

-- keyset 分页复合索引（featured → sort → name → id，与 ListPublic 排序一致）
CREATE INDEX IF NOT EXISTS idx_applications_list_order
    ON applications (is_featured, sort, name, id);
