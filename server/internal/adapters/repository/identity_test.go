package repository

import (
	"errors"
	"testing"
)

func TestEnsurePasswordState(t *testing.T) {
	if err := ensurePasswordState("same-hash", "same-hash"); err != nil {
		t.Fatalf("matching state rejected: %v", err)
	}
	if err := ensurePasswordState("stale-hash", "current-hash"); !errors.Is(err, ErrPasswordStateChanged) {
		t.Fatalf("expected concurrent state error, got %v", err)
	}
}
