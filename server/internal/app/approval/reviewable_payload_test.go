package approval

import "testing"

func TestValidateReviewablePayloadRejectsSecretsRecursively(t *testing.T) {
	for _, payload := range []string{
		`{"password":"not-for-approval"}`,
		`{"provider":{"client_secret":"not-for-approval"}}`,
		`{"items":[{"access-token":"not-for-approval"}]}`,
	} {
		canonical, err := canonicalJSON([]byte(payload))
		if err != nil {
			t.Fatalf("canonicalJSON() error = %v", err)
		}
		if validateReviewablePayload(canonical) == nil {
			t.Fatalf("validateReviewablePayload() accepted %s", payload)
		}
	}
}

func TestValidateReviewablePayloadAllowsSafeCommand(t *testing.T) {
	canonical, err := canonicalJSON([]byte(`{"force_change":true,"roles":["auditor"]}`))
	if err != nil {
		t.Fatalf("canonicalJSON() error = %v", err)
	}
	if err := validateReviewablePayload(canonical); err != nil {
		t.Fatalf("validateReviewablePayload() error = %v", err)
	}
}

func TestValidateReviewablePayloadRequiresObject(t *testing.T) {
	canonical, err := canonicalJSON([]byte(`["not-an-object"]`))
	if err != nil {
		t.Fatalf("canonicalJSON() error = %v", err)
	}
	if validateReviewablePayload(canonical) == nil {
		t.Fatal("validateReviewablePayload() accepted a non-object payload")
	}
}
