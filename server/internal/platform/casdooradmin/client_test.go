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
	client, err := New(Config{BaseURL: server.URL, Token: "token", Owner: "admin", Organization: "built-in", Enabled: true})
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
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode application request: %v", err)
		}
		for _, field := range []string{"providers", "signupItems", "signinItems", "tags", "samlAttributes", "tokenFields"} {
			value, ok := request[field].([]any)
			if !ok || value == nil || len(value) != 0 {
				t.Fatalf("%s must be encoded as an empty array, got %#v", field, request[field])
			}
		}
		if request["enableSigninSession"] != true || request["enableAutoSignin"] != true {
			t.Fatalf("OIDC application must reuse the established identity session: %#v", request)
		}
		if request["owner"] != "admin" || request["organization"] != "built-in" || request["clientSecret"] == "" {
			t.Fatalf("provider ownership or generated client secret is missing: %#v", request)
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":{"name":"demo","scopes":["openid","profile"]}}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: "token", Owner: "admin", Organization: "built-in", Enabled: true})
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
	if got := application.TakeOneTimeClientSecret(); len(got) < 32 {
		t.Fatalf("generated secret is too short: %d", len(got))
	}
	if got := application.TakeOneTimeClientSecret(); got != "" {
		t.Fatalf("secret was not cleared: %q", got)
	}
	encoded, err := json.Marshal(application)
	if err != nil || strings.Contains(string(encoded), "one-time-secret") {
		t.Fatalf("secret serialized: %s", encoded)
	}
}

func TestUpdateUsesOwnerQualifiedIDAndColumnPatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/get-application" {
			if r.URL.Query().Get("id") != "admin/demo" {
				t.Fatalf("unqualified provider id: %q", r.URL.Query().Get("id"))
			}
			_, _ = w.Write([]byte(`{"status":"ok","data":{"owner":"admin","name":"demo","organization":"built-in","clientId":"old","redirectUris":["https://old.example/callback"],"grantTypes":["authorization_code"],"enableSigninSession":true}}`))
			return
		}
		if r.URL.Path != "/api/update-application" || r.URL.Query().Get("id") != "admin/demo" || !strings.Contains(r.URL.Query().Get("columns"), "redirectUris") {
			t.Fatalf("unsafe update request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":true}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Token: "token", Owner: "admin", Organization: "built-in", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	application, created, err := client.UpsertApplication(context.Background(), UpsertInput{Name: "demo", ClientID: "new", RedirectURIs: []string{"https://new.example/callback"}, GrantTypes: []string{"authorization_code"}, Scopes: []string{"openid"}, ApprovalID: "approval-1"})
	if err != nil || created || requests != 2 || application.ClientID != "new" || application.TakeOneTimeClientSecret() != "" {
		t.Fatalf("update = %#v, created=%v requests=%d err=%v", application, created, requests, err)
	}
}
