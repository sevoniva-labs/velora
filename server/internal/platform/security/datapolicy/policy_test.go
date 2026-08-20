package datapolicy

import "testing"

func TestCatalogRequiresControlsForPersonalInformation(t *testing.T) {
	if _, err := NewCatalog([]FieldPolicy{{
		Key: "customer.mobile", Classification: ClassificationPersonalInformation,
		Owner: "retail", Purpose: "customer service", Residency: "cn", RetentionDays: 365, Mask: MaskNone,
	}}); err == nil {
		t.Fatal("personal information without masking was accepted")
	}
	catalog, err := NewCatalog([]FieldPolicy{{
		Key: "customer.mobile", Classification: ClassificationPersonalInformation,
		Owner: "retail", Purpose: "customer service", Residency: "cn", RetentionDays: 365, Mask: MaskMobile,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := catalog.MaskValue("customer.mobile", "13812345678"); err != nil || got != "138****5678" {
		t.Fatalf("MaskValue() = %q, %v", got, err)
	}
	if err := catalog.AuthorizeExport([]string{"customer.mobile"}, ExportRequest{Purpose: "case review"}); err == nil {
		t.Fatal("sensitive export without approval was accepted")
	}
	if err := catalog.AuthorizeExport([]string{"customer.mobile"}, ExportRequest{Purpose: "case review", ApprovalID: "approval-1", Watermark: "case-1"}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRejectsUnknownAndUnregisteredFields(t *testing.T) {
	if _, err := NewCatalog([]FieldPolicy{{
		Key: "x", Classification: "unknown", Owner: "team", Purpose: "test", Residency: "cn", RetentionDays: 1, Mask: MaskNone,
	}}); err == nil {
		t.Fatal("unknown classification was accepted")
	}
	catalog, err := NewCatalog([]FieldPolicy{{
		Key: "x", Classification: ClassificationPublic, Owner: "team", Purpose: "test", Residency: "cn", RetentionDays: 1, Mask: MaskNone,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.MaskValue("missing", "value"); err == nil {
		t.Fatal("unregistered field was accepted")
	}
}
