package kratosapi

import (
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	appconfigchange "github.com/sevoniva-labs/velora/server/internal/app/configchange"
)

func TestConfigChangeInteractiveBoundaryIsNotReportedAsServerFailure(t *testing.T) {
	err := kratoserrors.FromError(serviceError(appconfigchange.ErrActorRequired))
	if err.Code != 403 || err.Reason != "INTERACTIVE_SESSION_REQUIRED" {
		t.Fatalf("unexpected error mapping: code=%d reason=%s", err.Code, err.Reason)
	}
}
