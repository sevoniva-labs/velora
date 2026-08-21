package scheduler

import (
	"context"
	"log/slog"
	"time"

	lockx "github.com/sevoniva-labs/velora/server/internal/platform/lock"
)

type Job func(context.Context) error

type Runner struct {
	locks *lockx.Manager
	log   *slog.Logger
}

func New(locks *lockx.Manager, log *slog.Logger) *Runner { return &Runner{locks: locks, log: log} }

// RunDistributed executes a periodic job on at most one healthy instance when
// the lock backend is Redis. With memory cache it is single-process only.
func (r *Runner) RunDistributed(ctx context.Context, name string, every, lease time.Duration, job Job) {
	if every <= 0 {
		return
	}
	if lease <= 0 {
		lease = every
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			release, err := r.locks.Acquire(ctx, "job:"+name, lease)
			if err != nil {
				if err != lockx.ErrBusy {
					r.log.Error("scheduled job lock", "job", name, "err", err)
				}
				continue
			}
			if err := job(ctx); err != nil {
				r.log.Error("scheduled job", "job", name, "err", err)
			}
			_ = release(ctx)
		}
	}
}
