package kratosapi

import (
	"testing"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestApiTokenProtoDoesNotExposeHashOrSecret(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(24 * time.Hour)
	token := apiTokenProto(domain.APIToken{
		ID: "token-1", Name: "settlement-job", Prefix: "forge_ab12", Scopes: []string{"system.audit.read"},
		CreatedAt: now, ExpiresAt: &expires,
	})
	if token.Id != "token-1" || token.Prefix != "forge_ab12" || token.ExpiresAt.AsTime() != expires {
		t.Fatalf("unexpected token mapping: %+v", token)
	}
}
