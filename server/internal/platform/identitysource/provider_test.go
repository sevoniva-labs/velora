package identitysource

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
)

func TestNewLDAPProviderRequiresSecureTransportByDefault(t *testing.T) {
	_, err := NewLDAPProvider(LDAPConfig{Name: "ad", URL: "ldap://directory.example", BindDN: "cn=svc", BaseDN: "dc=example,dc=com", LoginAttribute: "sAMAccountName"})
	if err != ErrInvalidConfiguration {
		t.Fatalf("NewLDAPProvider() error = %v", err)
	}
}

func TestNewLDAPProviderAppliesSafeDefaults(t *testing.T) {
	provider, err := NewLDAPProvider(LDAPConfig{Name: "ad", URL: "ldaps://directory.example", BindDN: "cn=svc", BaseDN: "dc=example,dc=com", LoginAttribute: "sAMAccountName"})
	if err != nil {
		t.Fatalf("NewLDAPProvider() error = %v", err)
	}
	if provider.cfg.SearchTimeout != 5*time.Second || provider.cfg.DisplayAttribute != "displayName" || provider.cfg.GroupAttribute != "memberOf" {
		t.Fatalf("unexpected LDAP defaults: %#v", provider.cfg)
	}
	filter := fmt.Sprintf(provider.cfg.UserFilter, ldap.EscapeFilter(provider.cfg.LoginAttribute), ldap.EscapeFilter("alice"))
	if filter != "(&(objectClass=person)(sAMAccountName=alice))" {
		t.Fatalf("LDAP user filter = %q", filter)
	}
}

func TestUniqueStringsRemovesEmptyAndDuplicates(t *testing.T) {
	got := uniqueStrings([]string{" user ", "", "user", "auditor"})
	if len(got) != 2 || got[0] != "user" || got[1] != "auditor" {
		t.Fatalf("uniqueStrings() = %#v", got)
	}
}

func TestNewOIDCProviderRequiresConfidentialClientConfiguration(t *testing.T) {
	_, err := NewOIDCProvider(context.Background(), nil, OIDCConfig{
		Name: "casdoor", Issuer: "https://casdoor.example.com", ClientID: "velora", RedirectURL: "https://velora.example.com/api/v1/auth/federated/oidc/casdoor/callback",
	})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("NewOIDCProvider() error = %v, want ErrInvalidConfiguration", err)
	}
}
