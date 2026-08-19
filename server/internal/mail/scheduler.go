package mail

import (
	"context"
	"log/slog"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
)

// Scheduler 定时补偿同步：不把可靠性押在长连接上，
// 周期性对全部启用同步的账号做增量 reconciliation（IMAP IDLE 属 Phase 2）。
//
// 多实例互斥（DB lease）：多个 server 副本同时部署时，只有持有 lease 的实例执行同步，
// 避免重复拉取邮件。lease 通过 mail_sync_lease 表实现（见 0004 迁移），
// 过期时间 = 2×interval；实例崩溃后 lease 自动过期，其他实例接管。
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

	// 首次 tick 前的竞态防护：启动后先尝试抢一次 lease，抢不到则等下一轮。
	// 每轮：尝试抢 lease（幂等，过期自动续）→ 抢到才同步。
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.releaseLease()
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(parent context.Context) {
	// lease 生命周期：当前轮同步 + 余量；同步本身受 5 分钟超时保护。
	syncCtx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()

	acquired, err := s.svc.AcquireSyncLease(syncCtx)
	if err != nil {
		slog.Warn("邮件同步 lease 检查失败", "error", err)
		return
	}
	if !acquired {
		slog.Debug("邮件同步 lease 被其他实例持有，跳过本轮")
		return
	}
	defer s.svc.ReleaseSyncLease(syncCtx)

	start := time.Now()
	n, err := s.svc.SyncAll(syncCtx)
	metrics.Observe("velora_mail_sync_duration_milliseconds", float64(time.Since(start).Milliseconds()))
	if err != nil {
		metrics.Emit("velora_mail_sync_failure_total")
		slog.Warn("邮件定时同步失败", "error", err)
		return
	}
	metrics.Emit("velora_mail_sync_total")
	if n > 0 {
		slog.Info("邮件定时同步完成", "accounts", n)
	}
}

func (s *Scheduler) releaseLease() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.svc.ReleaseSyncLease(ctx); err != nil {
		slog.Warn("邮件同步 lease 释放失败", "error", err)
	}
}
