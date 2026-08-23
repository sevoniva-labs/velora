package velora

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProvisioningHandlerRejectsBadSignatureAndAcknowledgesDuplicate(t *testing.T) {
	secret := strings.Repeat("s", 32)
	handler, err := NewProvisioningHandler(secret, NewMemoryProvisioningStore())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"schema_version":"1.0","event_id":"event-1","event_type":"integration.challenge","aggregate_version":2,"occurred_at":"2027-01-15T08:00:00Z","source":"velora","challenge":{"application_code":"reference","challenge_id":"challenge-1"}}`)
	request := func(signature string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/provisioning", bytes.NewReader(body))
		req.Header.Set("X-Velora-Timestamp", strconv.FormatInt(now.Unix(), 10))
		req.Header.Set("X-Velora-Signature", signature)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	if got := request("v1=bad"); got.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature status=%d", got.Code)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(now.Unix(), 10) + "."))
	_, _ = mac.Write(body)
	signature := "v1=" + hex.EncodeToString(mac.Sum(nil))
	if got := request(signature); got.Code != 200 || !strings.Contains(got.Body.String(), "APPLIED") {
		t.Fatalf("first=%d %s", got.Code, got.Body.String())
	}
	if got := request(signature); got.Code != 200 || !strings.Contains(got.Body.String(), "DUPLICATE") {
		t.Fatalf("duplicate=%d %s", got.Code, got.Body.String())
	}
}
