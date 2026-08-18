package main

import (
	"context"
	"fmt"
	"log/slog"

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
	slog.Info("Seed 完成：分类与标签（示例应用已移除，门户应用请从 Casdoor 同步或管理后台创建）")
	return nil
}
