package password

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestVerifyRejectsOutOfRangeArgonParameters(t *testing.T) {
	b64 := base64.RawStdEncoding
	salt := b64.EncodeToString(make([]byte, 16))
	key := b64.EncodeToString(make([]byte, 32))
	encoded := fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=256$%s$%s", salt, key)
	if DefaultHasher().Verify("secret", encoded) {
		t.Fatal("parallelism that cannot fit uint8 was accepted")
	}
}
