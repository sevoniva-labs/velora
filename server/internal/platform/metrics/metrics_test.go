package metrics

import (
	"strings"
	"testing"
)

func TestRegisterAndRender(t *testing.T) {
	RegisterCounter("test_metric_total", "test help")
	Emit("test_metric_total")
	Emit("test_metric_total")

	out := string(Render(nil))
	if !strings.Contains(out, "test_metric_total 2") {
		t.Fatalf("expected counter value 2, got:\n%s", out)
	}
	if !strings.Contains(out, "# HELP test_metric_total test help") {
		t.Fatalf("missing help text")
	}
	if !strings.Contains(out, "# TYPE test_metric_total counter") {
		t.Fatalf("missing type")
	}
}

func TestHistogramBuckets(t *testing.T) {
	RegisterHistogram("test_dur_milliseconds", "test hist")
	Observe("test_dur_milliseconds", 30)
	Observe("test_dur_milliseconds", 3000)

	out := string(Render(nil))
	if !strings.Contains(out, "test_dur_milliseconds_bucket{le=\"50\"} 1") {
		t.Fatalf("expected 1 in le=50 bucket, got:\n%s", out)
	}
	if !strings.Contains(out, "test_dur_milliseconds_bucket{le=\"5000\"} 2") {
		t.Fatalf("expected 2 in le=5000 bucket")
	}
	if !strings.Contains(out, "test_dur_milliseconds_count 2") {
		t.Fatalf("expected count 2")
	}
	if !strings.Contains(out, "test_dur_milliseconds_sum 3030") {
		t.Fatalf("expected sum 3030")
	}
}

func TestRuntimeAndDBStats(t *testing.T) {
	out := string(Render(func() (int64, int64, int64, bool) {
		return 5, 2, 3, true
	}))
	for _, want := range []string{
		"velora_up 1",
		"velora_go_goroutines ",
		"velora_db_connections_open 5",
		"velora_db_connections_in_use 2",
		"velora_db_connections_idle 3",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestRegisterIdempotent(t *testing.T) {
	RegisterCounter("idem_total", "first")
	RegisterCounter("idem_total", "second")
	Emit("idem_total")
	out := string(Render(nil))
	if !strings.Contains(out, "# HELP idem_total first") {
		t.Fatalf("first help should be kept")
	}
	if strings.Contains(out, "# HELP idem_total second") {
		t.Fatalf("second help should be ignored")
	}
}

func TestUnknownEmitNoPanic(t *testing.T) {
	Emit("never_registered_total") // 不应 panic
	Observe("never_registered_hist", 1)
}
