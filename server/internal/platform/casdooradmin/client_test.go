package casdooradmin

import (
	"context"
	"encoding/json"
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

func TestUpsertReturnsClientSecretOnlyOnceInMemory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/get-application" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":{"name":"demo","scopes":["openid","profile"],"clientSecret":"one-time-secret"}}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: "token", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	application, created, err := client.UpsertApplication(context.Background(), UpsertInput{Name: "demo", Organization: "built-in", RedirectURIs: []string{"https://app.example.test/callback"}, Scopes: []string{"openid", "profile"}, ApprovalID: "approval-1"})
	if err != nil || !created {
		t.Fatalf("UpsertApplication() = created %t, err %v", created, err)
	}
	if len(application.Scopes) != 2 || application.Scopes[0] != "openid" {
		t.Fatalf("scopes were not returned: %#v", application.Scopes)
	}
	if got := application.TakeOneTimeClientSecret(); got != "one-time-secret" {
		t.Fatalf("first secret = %q", got)
	}
	if got := application.TakeOneTimeClientSecret(); got != "" {
		t.Fatalf("secret was not cleared: %q", got)
	}
	encoded, err := json.Marshal(application)
	if err != nil || strings.Contains(string(encoded), "one-time-secret") {
		t.Fatalf("secret serialized: %s", encoded)
	}
}
