package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSPAStaticIndexResourceKeepsExactSignedURL(t *testing.T) {
	root := t.TempDir()
	resource := filepath.Join(root, "microapps", "example-remote", "1.0.0", "index.html")
	if err := os.MkdirAll(filepath.Dir(resource), 0o750); err != nil {
		t.Fatalf("create resource directory: %v", err)
	}
	if err := os.WriteFile(resource, []byte("signed-resource"), 0o600); err != nil {
		t.Fatalf("write signed resource: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/microapps/example-remote/1.0.0/index.html", nil)
	response := httptest.NewRecorder()
	SPA(SPAOptions{Root: root}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if location := response.Header().Get("Location"); location != "" {
		t.Fatalf("unexpected canonical redirect to %q", location)
	}
	if body := response.Body.String(); body != "signed-resource" {
		t.Fatalf("body = %q, want signed resource", body)
	}
}
