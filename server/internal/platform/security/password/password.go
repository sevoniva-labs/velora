package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
)

type Policy struct {
	MinLength     int
	RequireUpper  bool
	RequireLower  bool
	RequireDigit  bool
	RequireSymbol bool
}

type Hasher struct {
	MemoryKiB   uint32
	Time        uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultHasher() Hasher {
	return Hasher{MemoryKiB: 64 * 1024, Time: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32}
}

func (p Policy) Validate(raw string) error {
	if len([]rune(raw)) < p.MinLength {
		return fmt.Errorf("password must be at least %d characters", p.MinLength)
	}
	if len([]rune(raw)) > 128 {
		return errors.New("password is too long")
	}
	var upper, lower, digit, symbol bool
	for _, r := range raw {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	if p.RequireUpper && !upper {
		return errors.New("password requires uppercase character")
	}
	if p.RequireLower && !lower {
		return errors.New("password requires lowercase character")
	}
	if p.RequireDigit && !digit {
		return errors.New("password requires digit")
	}
	if p.RequireSymbol && !symbol {
		return errors.New("password requires symbol")
	}
	return nil
}

func (h Hasher) Hash(raw string) (string, error) {
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(raw), salt, h.Time, h.MemoryKiB, h.Parallelism, h.KeyLength)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", h.MemoryKiB, h.Time, h.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func (h Hasher) Verify(raw, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t, p uint64
	for _, kv := range strings.Split(parts[3], ",") {
		x := strings.SplitN(kv, "=", 2)
		if len(x) != 2 {
			return false
		}
		n, e := strconv.ParseUint(x[1], 10, 32)
		if e != nil {
			return false
		}
		switch x[0] {
		case "m":
			m = n
		case "t":
			t = n
		case "p":
			p = n
		}
	}
	b64 := base64.RawStdEncoding
	salt, e := b64.DecodeString(parts[4])
	if e != nil {
		return false
	}
	expected, e := b64.DecodeString(parts[5])
	if e != nil {
		return false
	}
	if m == 0 || m > math.MaxUint32 || t == 0 || t > math.MaxUint32 || p == 0 || p > math.MaxUint8 || len(salt) < 8 || len(salt) > 1024 || len(expected) == 0 || len(expected) > 1024 {
		return false
	}
	// #nosec G115 -- every narrowing conversion is bounded immediately above.
	actual := argon2.IDKey([]byte(raw), salt, uint32(t), uint32(m), uint8(p), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
