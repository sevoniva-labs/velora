package httpserver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpx"
)

func TestEncodeResponseUsesEnvelopeSnakeCaseAndListCompatibility(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/admin/roles", nil)
	response := httptest.NewRecorder()
	err := EncodeResponse(response, request, &forgev1.ListRolesResponse{Roles: []*forgev1.Role{{Key: "auditor", DataScope: "ORGANIZATION"}}})
	if err != nil {
		t.Fatalf("EncodeResponse() error = %v", err)
	}
	var envelope struct {
		Code string                     `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != "000000" || envelope.Data["items"] == nil || envelope.Data["roles"] == nil {
		t.Fatalf("unexpected envelope: %s", response.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(envelope.Data["items"], &items); err != nil || len(items) != 1 || items[0]["key"] != "auditor" || items[0]["data_scope"] != "ORGANIZATION" {
		t.Fatalf("unexpected items: %s", envelope.Data["items"])
	}
}

func TestEncodeResponseUnwrapsSingleMessage(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/me", nil)
	response := httptest.NewRecorder()
	if err := EncodeResponse(response, request, &forgev1.GetCurrentUserResponse{User: &forgev1.User{Id: "user-1"}}); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["id"] != "user-1" {
		t.Fatalf("single message was not unwrapped: %s", response.Body.String())
	}
}

func TestEncodeErrorUsesSafeEnvelope(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/v1/approvals", nil)
	response := httptest.NewRecorder()
	EncodeError(response, request, kratoserrors.Forbidden("STEP_UP_REQUIRED", "recent MFA required"))
	var envelope httpx.Envelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if response.Code != 403 || envelope.Error != "STEP_UP_REQUIRED" || envelope.Code == "000000" {
		t.Fatalf("unexpected error envelope: %s", response.Body.String())
	}
}
