package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sevoniva-labs/velora/server/internal/application"
	"github.com/sevoniva-labs/velora/server/internal/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/db"
)

// seed 写入开发 Seed 数据：分类 + 示例应用（全部 EVERYONE 可见，无真实企业信息）。
func seed(cfg *config.Config) error {
	ctx := context.Background()
	gormDB, err := db.Connect(cfg)
	if err != nil {
		return err
	}
	if err := db.Migrate(ctx, gormDB); err != nil {
		return err
	}

	// --- 分类 ---
	categories := []struct {
		Code string
		Name string
		Desc string
		Sort int
	}{
		{"rd", "研发工具", "研发协作与开发工具", 1},
		{"project", "项目管理", "项目与任务管理", 2},
		{"qa", "测试工具", "测试与质量保障", 3},
		{"ops", "运维平台", "运维与监控平台", 4},
		{"data", "数据平台", "数据开发与 BI 平台", 5},
		{"ai", "AI 工具", "AI 与智能应用", 6},
		{"office", "办公协作", "办公与协同办公", 7},
		{"other", "其他", "其他系统", 8},
	}
	catID := map[string]uint64{}
	for _, c := range categories {
		var id uint64
		err := gormDB.Raw(
			`INSERT INTO application_categories (code, name, description, sort)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, sort = EXCLUDED.sort
			 RETURNING id`, c.Code, c.Name, c.Desc, c.Sort,
		).Scan(&id).Error
		if err != nil {
			return fmt.Errorf("写入分类 %s 失败: %w", c.Code, err)
		}
		catID[c.Code] = id
	}
	slog.Info("分类 Seed 完成", "count", len(categories))

	// --- 标签 ---
	tags := []struct {
		Code string
		Name string
		Sort int
	}{
		{"ci", "CI/CD", 1},
		{"code", "代码托管", 2},
		{"monitor", "监控告警", 3},
		{"bi", "BI 报表", 4},
		{"genai", "生成式 AI", 5},
		{"im", "即时通讯", 6},
	}
	tagID := map[string]uint64{}
	for _, t := range tags {
		var id uint64
		err := gormDB.Raw(
			`INSERT INTO application_tags (code, name, sort)
			 VALUES (?, ?, ?)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, sort = EXCLUDED.sort
			 RETURNING id`, t.Code, t.Name, t.Sort,
		).Scan(&id).Error
		if err != nil {
			return fmt.Errorf("写入标签 %s 失败: %w", t.Code, err)
		}
		tagID[t.Code] = id
	}
	slog.Info("标签 Seed 完成", "count", len(tags))

	// --- 示例应用（占位地址，非真实企业信息） ---
	apps := []struct {
		Code     string
		Name     string
		Desc     string
		Keywords string
		Icon     string
		Category string
		HomeURL  string
		SSOType  string
		Featured bool
		Tags     []string
		Health   string
	}{
		{"devops", "DevOps 平台", "统一 DevOps 流水线、制品与发布平台", "devops,流水线,发布", "🚀", "ops", "https://devops.example.internal", application.SSOTypeURL, true, []string{"ci"}, ""},
		{"git", "Git Repository", "企业级代码托管与代码评审", "git,代码托管,仓库", "🧬", "rd", "https://git.example.internal", application.SSOTypeURL, true, []string{"code"}, "https://git.example.internal/healthz"},
		{"artifact", "Artifact Repository", "制品仓库：Maven / npm / PyPI 等", "artifact,制品,仓库,包管理", "📦", "rd", "https://artifact.example.internal", application.SSOTypeURL, false, []string{"ci"}, ""},
		{"test", "Test Platform", "自动化测试与质量门禁平台", "test,测试,自动化", "🧪", "qa", "https://test.example.internal", application.SSOTypeURL, false, []string{"ci"}, ""},
		{"data", "Data Platform", "数据开发、调度与 BI 报表平台", "data,数据,BI,报表", "📊", "data", "https://data.example.internal", application.SSOTypeURL, false, []string{"bi"}, ""},
		{"ai", "AI Platform", "企业 AI 平台与智能助手", "ai,智能,大模型", "🤖", "ai", "https://ai.example.internal", application.SSOTypeOIDC, true, []string{"genai"}, ""},
		{"im", "企业 IM", "企业内部即时通讯与协同", "im,即时通讯,协同", "💬", "office", "https://im.example.internal", application.SSOTypeURL, false, []string{"im"}, ""},
		{"monitor", "监控平台", "基础设施与应用监控告警", "monitor,监控,告警", "📡", "ops", "https://monitor.example.internal", application.SSOTypeURL, false, []string{"monitor"}, "https://monitor.example.internal/healthz"},
	}

	for _, a := range apps {
		var appID uint64
		var catIDVal any
		if v, ok := catID[a.Category]; ok {
			catIDVal = v
		} else {
			catIDVal = nil
		}
		err := gormDB.Raw(
			`INSERT INTO applications
			   (code, name, description, keywords, icon, category_id, home_url, launch_url, sso_type,
			    owner, department, status, sort, is_featured, health_check_enabled, health_check_url, created_by, updated_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', 'ENABLED', 0, ?, ?, ?, 'seed', 'seed')
			 ON CONFLICT (code) DO UPDATE SET
			   name = EXCLUDED.name, description = EXCLUDED.description, keywords = EXCLUDED.keywords,
			   icon = EXCLUDED.icon, category_id = EXCLUDED.category_id, home_url = EXCLUDED.home_url,
			   launch_url = EXCLUDED.launch_url, sso_type = EXCLUDED.sso_type, is_featured = EXCLUDED.is_featured,
			   health_check_enabled = EXCLUDED.health_check_enabled, health_check_url = EXCLUDED.health_check_url
			 RETURNING id`,
			a.Code, a.Name, a.Desc, a.Keywords, a.Icon, catIDVal, a.HomeURL, a.HomeURL, a.SSOType,
			a.Featured, a.Health != "", a.Health,
		).Scan(&appID).Error
		if err != nil {
			return fmt.Errorf("写入应用 %s 失败: %w", a.Code, err)
		}

		// 标签关系。
		for _, t := range a.Tags {
			if id, ok := tagID[t]; ok {
				if err := gormDB.Exec(
					`INSERT INTO application_tag_relations (application_id, tag_id) VALUES (?, ?)
					 ON CONFLICT DO NOTHING`, appID, id,
				).Error; err != nil {
					return fmt.Errorf("写入应用标签失败: %w", err)
				}
			}
		}
		// 默认 EVERYONE 策略（首次插入时）。
		var polCount int64
		if err := gormDB.Table("application_access_policies").
			Where("application_id = ?", appID).Count(&polCount).Error; err != nil {
			return err
		}
		if polCount == 0 {
			if err := gormDB.Exec(
				`INSERT INTO application_access_policies (application_id, policy_type, value, created_at, updated_at)
				 VALUES (?, 'EVERYONE', '', now(), now())`, appID,
			).Error; err != nil {
				return err
			}
		}
	}
	slog.Info("应用 Seed 完成", "count", len(apps))
	return nil
}
