// Package db 提供 GORM 初始化与极简 SQL 迁移执行器。
//
// 迁移约定：
//   - migrations/ 目录下按文件名字典序执行（如 0001_init.sql）
//   - schema_migrations 表记录已执行文件名，保证幂等
package db

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/sevoniva-labs/velora/server/internal/config"
	"github.com/sevoniva-labs/velora/server/migrations"
)

// MigrationFS 引用嵌入的迁移 SQL 文件。
var MigrationFS = migrations.FS

// Connect 建立 PostgreSQL 连接并配置连接池。
func Connect(cfg *config.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}
	if cfg.Env == "development" {
		gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Info)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("PostgreSQL 连通性检查失败: %w", err)
	}
	return db, nil
}

// Migrate 执行 migrations/ 下尚未应用的 SQL 文件。
// 通过 pg_advisory_xact_lock 串行化，防止多实例并发启动时迁移冲突
// （advisory 锁随事务自动释放，无需手动解锁）。
func Migrate(ctx context.Context, db *gorm.DB) error {
	// 迁移专用全局锁键（固定常量）。
	const lockKey = 0x56454C4F4D494752 // "VELOMIGR"
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("SELECT pg_advisory_xact_lock($1)", lockKey).Error; err != nil {
			return fmt.Errorf("无法获取迁移锁: %w", err)
		}
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`).Error; err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	files, err := fs.Glob(MigrationFS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, file := range files {
		name := file
		var count int64
		if err := db.Table("schema_migrations").Where("filename = ?", name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		sqlBytes, err := MigrationFS.ReadFile(file)
		if err != nil {
			return err
		}
		slog.Info("应用迁移", "file", name)
		// 每个迁移文件在事务中执行。
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range splitStatements(string(sqlBytes)) {
				if strings.TrimSpace(stmt) == "" {
					continue
				}
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("迁移 %s 失败: %w", name, err)
				}
			}
			return tx.Table("schema_migrations").Create(map[string]any{
				"filename": name,
			}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

// splitStatements 按分号切分 SQL 语句（不处理过程体内的分号，本项目迁移均为简单 DDL）。
func splitStatements(sql string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	return out
}
