package health

import (
	"context"
	"sync"
	"time"
)

type Check struct {
	Name, Provider string
	Ping           func(context.Context) error
}
type Result struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

func Run(ctx context.Context, checks []Check) []Result {
	out := make([]Result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func(i int, c Check) {
			defer wg.Done()
			start := time.Now()
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			e := c.Ping(cctx)
			r := Result{Name: c.Name, Provider: c.Provider, Status: "UP", DurationMS: time.Since(start).Milliseconds()}
			if e != nil {
				r.Status = "DOWN"
				r.Error = e.Error()
			}
			out[i] = r
		}(i, c)
	}
	wg.Wait()
	return out
}
