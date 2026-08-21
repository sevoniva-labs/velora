package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Customer  string           `json:"customer"`
	Plan      string           `json:"plan"`
	Features  map[string]bool  `json:"features"`
	Limits    map[string]int64 `json:"limits"`
	IssuedAt  time.Time        `json:"issued_at"`
	ExpiresAt time.Time        `json:"expires_at"`
}

func (c Claims) Enabled(feature string) bool { return c.Features[feature] }
func (c Claims) Limit(key string) int64      { return c.Limits[key] }
func (c Claims) ValidAt(t time.Time) bool {
	return !c.ExpiresAt.IsZero() && t.Before(c.ExpiresAt) && !t.Before(c.IssuedAt)
}

// VerifyOffline verifies a compact "base64url(payload).base64url(signature)"
// Ed25519 license. Private signing keys never belong in the deployed product.
func VerifyOffline(raw string, pub ed25519.PublicKey, now time.Time) (Claims, error) {
	var c Claims
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return c, errors.New("license format invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, err
	}
	if !ed25519.Verify(pub, payload, sig) {
		return c, errors.New("license signature invalid")
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, err
	}
	if !c.ValidAt(now) {
		return c, errors.New("license expired or not active")
	}
	return c, nil
}
