package provisioninghttp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
)

func TestPublishSignsExactBody(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		timestamp := r.Header.Get("X-Velora-Timestamp")
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "."))
		_, _ = mac.Write(body)
		if r.Header.Get("X-Velora-Signature") != "v1="+hex.EncodeToString(mac.Sum(nil)) {
			t.Fatal("invalid signature")
		}
		_, _ = w.Write([]byte(`{"status":"APPLIED"}`))
	}))
	defer server.Close()
	d, err := New(Config{Enabled: true, URL: server.URL, Secret: secret, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Publish(context.Background(), messaging.Message{ID: "event-1", Topic: "velora.provisioning.spectra", Type: "user.entitlements.changed", Body: []byte(`{"event_id":"event-1"}`)}); err != nil {
		t.Fatal(err)
	}
}
