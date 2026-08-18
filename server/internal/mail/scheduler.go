package mail

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler 定时补偿同步：不把可靠性押在长连接上，
// 周期性对全部启用同步的账号做增量 reconciliation（IMAP IDLE 属 Phase 2）。
type Scheduler struct {
	svc      *Service
	interval time.Duration
}

// NewScheduler 创建同步调度器；interval <= 0 时 Run 直接退出（禁用）。
func NewScheduler(svc *Service, interval time.Duration) *Scheduler {
	return &Scheduler{svc: svc, interval: interval}
}

// Run 阻塞运行直到 ctx 取消；应在独立 goroutine 中启动。
func (s *Scheduler) Run(ctx context.Context) {
	if s.interval <= 0 {
		slog.Info("邮件定时同步已禁用")
		return
	}
	slog.Info("邮件定时同步已启动", "interval", s.interval.String())
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			n, err := s.svc.SyncAll(syncCtx)
			cancel()
			if err != nil {
				slog.Warn("邮件定时同步失败", "error", err)
			} else if n > 0 {
				slog.Info("邮件定时同步完成", "accounts", n)
			}
		}
	}
}
