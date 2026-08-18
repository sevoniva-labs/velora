// Velora 企业应用门户服务端入口。
//
// 用法：
//
//	velora serve   启动 HTTP 服务（启动前自动执行幂等迁移）
//	velora migrate 仅执行数据库迁移
//	velora seed    写入开发 Seed 数据（分类 + 示例应用）
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/db"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpserver"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置错误:", err)
		os.Exit(1)
	}
	setupLogger(cfg)

	var runErr error
	switch os.Args[1] {
	case "serve":
		runErr = serve(cfg)
	case "migrate":
		runErr = migrate(cfg)
	case "seed":
		runErr = seed(cfg)
	default:
		usage()
		os.Exit(1)
	}
	if runErr != nil {
		slog.Error("velora 退出", "error", runErr)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`Velora 企业应用门户服务端

用法:
  velora serve    启动 HTTP 服务（自动执行迁移）
  velora migrate  仅执行数据库迁移
  velora seed     写入开发 Seed 数据`)
}

func serve(cfg *config.Config) error {
	ctx := context.Background()

	gormDB, err := db.Connect(cfg)
	if err != nil {
		return err
	}
	if err := db.Migrate(ctx, gormDB); err != nil {
		return fmt.Errorf("迁移失败: %w", err)
	}

	sessions, err := auth.NewSessionStore(cfg.SessionSecret, cfg.SessionTTL, cfg.CookieSecure, cfg.CookieDomain)
	if err != nil {
		return err
	}
	oidc, err := auth.NewOIDCManager(ctx, cfg.CasdoorIssuer, cfg.CasdoorClientID, cfg.CasdoorClientSecret, cfg.CasdoorRedirectURI, 10*time.Minute)
	if err != nil {
		return err
	}

	auditSvc := audit.NewService(gormDB)
	engine, err := httpserver.New(httpserver.Deps{
		Cfg:       cfg,
		DB:        gormDB,
		Sessions:  sessions,
		OIDC:      oidc,
		Audit:     auditSvc,
		AdminRole: cfg.AdminRole,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("Velora server 启动", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常", "error", err)
		}
	}()

	// Graceful Shutdown。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("收到退出信号，开始优雅关闭…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("优雅关闭失败: %w", err)
	}
	slog.Info("服务已停止")
	return nil
}

func migrate(cfg *config.Config) error {
	gormDB, err := db.Connect(cfg)
	if err != nil {
		return err
	}
	if err := db.Migrate(context.Background(), gormDB); err != nil {
		return fmt.Errorf("迁移失败: %w", err)
	}
	slog.Info("数据库迁移完成")
	return nil
}

func setupLogger(cfg *config.Config) {
	var handler slog.Handler
	if cfg.Env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	switch cfg.LogLevel {
	case "debug":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	case "warn":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})
	case "error":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	}
	slog.SetDefault(slog.New(handler))
}
