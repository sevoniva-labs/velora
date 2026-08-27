package metrics

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObserveIdentityExportsBoundedFlowMetrics(t *testing.T) {
	m := New()
	m.ObserveIdentity("wechat_callback", "invalid_state", time.Now().Add(-10*time.Millisecond))
	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	output := string(body)
	for _, expected := range []string{
		`velora_identity_events_total{flow="wechat_callback",result="invalid_state"} 1`,
		`velora_identity_operation_duration_seconds_count{flow="wechat_callback"} 1`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("metrics output does not contain %q", expected)
		}
	}
}
