package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConstantEqual(t *testing.T) {
	if !constantEqual("state", "state") || constantEqual("state", "other") {
		t.Fatal("constantEqual contract failed")
	}
}

func TestSecurityHeadersAndHealth(t *testing.T) {
	d := &demo{issuer: "https://auth.example.test", tx: map[string]transaction{}, sessions: map[string]session{}}
	req := httptest.NewRequest(http.MethodGet, "https://demo.example.test/healthz", nil)
	res := httptest.NewRecorder()
	d.routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Header().Get("X-Content-Type-Options") != "nosniff" || res.Body.String() != "ok\n" {
		t.Fatalf("health response status=%d headers=%v body=%q", res.Code, res.Header(), res.Body.String())
	}
}
