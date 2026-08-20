package secrets

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/sevoniva-labs/velora/server/internal/platform/securefile"
)

// Provider is deliberately small so Vault, cloud KMS/Secrets Manager and
// domestic HSM/密码机 adapters can be added without changing business code.
type Provider interface {
	Get(context.Context, string) (string, error)
	Name() string
}

type EnvFile struct{ Prefix string }

func (p EnvFile) Name() string { return "env-file" }
func (p EnvFile) Get(_ context.Context, key string) (string, error) {
	name := p.Prefix + key
	if path := strings.TrimSpace(os.Getenv(name + "_FILE")); path != "" {
		b, err := securefile.Read(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	if v := os.Getenv(name); v != "" {
		return v, nil
	}
	return "", errors.New("secret not found")
}

type Chain []Provider

func (c Chain) Name() string { return "chain" }
func (c Chain) Get(ctx context.Context, key string) (string, error) {
	for _, p := range c {
		if v, err := p.Get(ctx, key); err == nil {
			return v, nil
		}
	}
	return "", errors.New("secret not found")
}
