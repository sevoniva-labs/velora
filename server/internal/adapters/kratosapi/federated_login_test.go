package kratosapi

import "testing"

func TestPKCEChallengeUsesS256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := pkceChallenge(verifier); got != want {
		t.Fatalf("pkceChallenge() = %q, want %q", got, want)
	}
	if got := pkceChallenge(verifier); got != pkceChallenge(verifier) {
		t.Fatal("pkceChallenge() is not deterministic")
	}
}

func TestOIDCTransactionKeyDoesNotExposeState(t *testing.T) {
	state := "state-with-high-entropy"
	key := oidcTransactionKey(state)
	if key == "oidc:state:"+state {
		t.Fatal("OIDC transaction key exposed raw state")
	}
	if len(key) != len("oidc:state:")+64 {
		t.Fatalf("unexpected transaction key length: %d", len(key))
	}
	if key != oidcTransactionKey(state) {
		t.Fatal("OIDC transaction key must be deterministic")
	}
}
