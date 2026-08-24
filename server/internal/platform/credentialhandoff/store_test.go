package credentialhandoff

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	appcrypto "github.com/sevoniva-labs/velora/server/internal/platform/security/crypto"
)

func TestHandoffIsSingleUseAndDoesNotStorePlainTokenOrSecret(t *testing.T) {
	c, err := cache.New(config.Cache{Provider: "memory", Prefix: "test:"})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := appcrypto.New("standard", "handoff-test-key-material-32-bytes")
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := appcrypto.NewEnvelopeCipher(provider)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(c, cipher)
	if err != nil {
		t.Fatal(err)
	}
	bundle := Bundle{ApplicationID: "app-1", ApplicationCode: "order-center", ClientSecret: strings.Repeat("c", 32), ProvisioningSecret: strings.Repeat("p", 32), DirectoryToken: "vd_" + strings.Repeat("d", 43)}
	token, _, err := store.Issue(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := c.Get(context.Background(), tokenKey(token))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, token) || strings.Contains(stored, bundle.ClientSecret) || strings.Contains(stored, bundle.ProvisioningSecret) {
		t.Fatal("handoff cache exposed plaintext credential material")
	}
	got, err := store.Consume(context.Background(), token)
	if err != nil || got.ApplicationCode != bundle.ApplicationCode {
		t.Fatalf("consume = %#v, %v", got, err)
	}
	if _, err := store.Consume(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second consume = %v", err)
	}
}
