package casdooradmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAutomationRequiresApprovalAndNeverSerializesSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "secret") {
			t.Fatalf("secret leaked into query")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","data":{"name":"demo"}}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: "token", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.UpsertApplication(context.Background(), UpsertInput{Name: "demo", Organization: "built-in"}); err != ErrApprovalRequired {
		t.Fatalf("expected approval error, got %v", err)
	}
}

func TestDisabledAutomationDoesNotCallRemote(t *testing.T) {
	client, err := New(Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if client.Enabled() {
		t.Fatal("automation unexpectedly enabled")
	}
	if _, _, err := client.GetApplication(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
}
