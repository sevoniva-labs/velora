package casdooridentity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
)

func TestCreateUserUsesBasicAuthAndReturnsStableSubject(t *testing.T) {
	created := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client" || pass != "secret" {
			t.Fatal("missing client authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/get-user":
			if !created {
				_, _ = w.Write([]byte(`{"status":"error","msg":"The user does not exist"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok","data":{"owner":"built-in","name":"carson","id":"subject-1"}}`))
		case "/api/add-user":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["password"] != "Strong#Password123" || body["name"] != "carson" {
				t.Fatalf("unexpected body: %#v", body)
			}
			created = true
			_, _ = w.Write([]byte(`{"status":"ok","data":"Affected"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, ClientID: "client", ClientSecret: "secret", Organization: "built-in", Application: "app-velora", Enabled: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	subject, err := client.CreateUser(context.Background(), appidentity.ManagedUserInput{LoginName: "carson", DisplayName: "Carson", Email: "carson@example.com", Password: "Strong#Password123"})
	if err != nil || subject != "subject-1" {
		t.Fatalf("CreateUser() = %q, %v", subject, err)
	}
}

func TestErrorDoesNotLeakCredentials(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret response", http.StatusBadGateway) }))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, ClientID: "client", ClientSecret: "top-secret", Organization: "built-in", Enabled: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateUser(context.Background(), appidentity.ManagedUserInput{LoginName: "carson", Password: "User#Secret123"})
	if err == nil || strings.Contains(err.Error(), "top-secret") || strings.Contains(err.Error(), "User#Secret123") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestCreateUserRecoversExactManagedPartialCreation(t *testing.T) {
	updated := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/get-user":
			_, _ = w.Write([]byte(`{"status":"ok","data":{"owner":"built-in","name":"carson","id":"subject-1","displayName":"Carson","email":"","signupApplication":"app-velora","isForbidden":true}}`))
		case "/api/update-user":
			if r.URL.Query().Get("columns") != "password,is_forbidden" {
				t.Fatalf("columns = %q", r.URL.Query().Get("columns"))
			}
			var body userWire
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Password != "Strong#Password123" || body.IsForbidden {
				t.Fatalf("unexpected recovery body: %#v", body)
			}
			updated = true
			_, _ = w.Write([]byte(`{"status":"ok","data":"Affected"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, ClientID: "client", ClientSecret: "secret", Organization: "built-in", Application: "app-velora", Enabled: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	subject, err := client.CreateUser(context.Background(), appidentity.ManagedUserInput{LoginName: "carson", DisplayName: "Carson", Password: "Strong#Password123"})
	if err != nil || subject != "subject-1" || !updated {
		t.Fatalf("CreateUser() = %q, %v, updated=%t", subject, err, updated)
	}
}

func TestSetUserStatusUsesVersionCompatibleColumnAndVerifiesPersistence(t *testing.T) {
	forbidden := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/get-user":
			_, _ = fmt.Fprintf(w, `{"status":"ok","data":{"owner":"built-in","name":"carson","id":"subject-1","displayName":"Carson","isForbidden":%t}}`, forbidden)
		case "/api/update-user":
			if r.URL.Query().Get("columns") != "is_forbidden" {
				t.Fatalf("columns = %q", r.URL.Query().Get("columns"))
			}
			var body userWire
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			forbidden = body.IsForbidden
			_, _ = w.Write([]byte(`{"status":"ok","data":"Affected"}`))
		case "/api/get-sessions", "/api/get-tokens":
			_, _ = w.Write([]byte(`{"status":"ok","data":[]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, ClientID: "client", ClientSecret: "secret", Organization: "built-in", Enabled: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetUserStatus(context.Background(), "carson", false); err != nil {
		t.Fatal(err)
	}
	if !forbidden {
		t.Fatal("user status was not persisted")
	}
}

func TestModifyRejectsUnchangedResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/get-user" {
			_, _ = w.Write([]byte(`{"status":"ok","data":{"owner":"built-in","name":"carson","id":"subject-1","displayName":"Carson"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":"Unchanged"}`))
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, ClientID: "client", ClientSecret: "secret", Organization: "built-in", Enabled: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetUserStatus(context.Background(), "carson", false); err == nil {
		t.Fatal("expected unchanged update to fail")
	}
}

func TestValidateWeChatPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  string
		wantErr string
	}{
		{name: "safe", policy: `{"name":"app-velora","enableSignUp":false,"enableLinkWithEmail":false,"providers":[{"name":"wechat-open","canSignIn":true,"canSignUp":false,"bindingRule":[]}]}`},
		{name: "application signup", policy: `{"name":"app-velora","enableSignUp":true,"providers":[]}`, wantErr: "sign-up must be disabled"},
		{name: "implicit auto link", policy: `{"name":"app-velora","providers":[{"name":"wechat-open","canSignIn":true,"canSignUp":false,"bindingRule":null}]}`, wantErr: "explicit empty array"},
		{name: "explicit auto link", policy: `{"name":"app-velora","providers":[{"name":"wechat-open","canSignIn":true,"canSignUp":false,"bindingRule":["Email"]}]}`, wantErr: "explicit empty array"},
		{name: "provider signup", policy: `{"name":"app-velora","providers":[{"name":"wechat-open","canSignIn":true,"canSignUp":true,"bindingRule":[]}]}`, wantErr: "provider sign-up must be disabled"},
		{name: "missing provider", policy: `{"name":"app-velora","providers":[]}`, wantErr: "not attached"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/get-application" || r.URL.Query().Get("id") != "admin/app-velora" {
					t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
				}
				user, pass, ok := r.BasicAuth()
				if !ok || user != "client" || pass != "secret" {
					t.Fatal("missing client authentication")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"status":"ok","data":%s}`, tt.policy)
			}))
			defer server.Close()
			client, err := New(Config{BaseURL: server.URL, ClientID: "client", ClientSecret: "secret", Organization: "built-in", Application: "app-velora", ApplicationOwner: "admin", Enabled: true, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			err = client.ValidateWeChatPolicy(context.Background(), "wechat-open")
			if tt.wantErr == "" && err != nil {
				t.Fatalf("ValidateWeChatPolicy() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("ValidateWeChatPolicy() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
