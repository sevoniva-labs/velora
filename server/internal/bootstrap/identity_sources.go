package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/sevoniva-labs/velora/server/internal/platform/identitysource"
)

func newFederatedIdentityProviders(ctx context.Context) (map[string]*identitysource.OIDCProvider, map[string]*identitysource.LDAPProvider, error) {
	oidcProviders := make(map[string]*identitysource.OIDCProvider)
	ldapProviders := make(map[string]*identitysource.LDAPProvider)
	if issuer := strings.TrimSpace(os.Getenv("VELORA_OIDC_ISSUER")); issuer != "" {
		name := strings.ToLower(env("VELORA_OIDC_NAME", "oidc"))
		allowHTTP, _ := strconv.ParseBool(env("VELORA_OIDC_ALLOW_HTTP", "false"))
		provider, err := identitysource.NewOIDCProvider(ctx, http.DefaultClient, identitysource.OIDCConfig{
			Name: name, Issuer: issuer, ClientID: strings.TrimSpace(os.Getenv("VELORA_OIDC_CLIENT_ID")), ClientSecret: secret("VELORA_OIDC_CLIENT_SECRET"),
			RedirectURL: strings.TrimSpace(os.Getenv("VELORA_OIDC_REDIRECT_URL")), AllowHTTP: allowHTTP,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("OIDC provider %q: %w", name, err)
		}
		oidcProviders[name] = provider
	}
	if url := strings.TrimSpace(os.Getenv("VELORA_LDAP_URL")); url != "" {
		name := strings.ToLower(env("VELORA_LDAP_NAME", "ldap"))
		startTLS, _ := strconv.ParseBool(env("VELORA_LDAP_STARTTLS", "false"))
		allowInsecure, _ := strconv.ParseBool(env("VELORA_LDAP_ALLOW_INSECURE", "false"))
		provider, err := identitysource.NewLDAPProvider(identitysource.LDAPConfig{
			Name: name, URL: url, BindDN: strings.TrimSpace(os.Getenv("VELORA_LDAP_BIND_DN")), BindPassword: secret("VELORA_LDAP_BIND_PASSWORD"),
			BaseDN: strings.TrimSpace(os.Getenv("VELORA_LDAP_BASE_DN")), UserFilter: strings.TrimSpace(os.Getenv("VELORA_LDAP_USER_FILTER")),
			LoginAttribute: env("VELORA_LDAP_LOGIN_ATTRIBUTE", "sAMAccountName"), DisplayAttribute: env("VELORA_LDAP_DISPLAY_ATTRIBUTE", "displayName"),
			EmailAttribute: env("VELORA_LDAP_EMAIL_ATTRIBUTE", "mail"), GroupAttribute: env("VELORA_LDAP_GROUP_ATTRIBUTE", "memberOf"),
			StartTLS: startTLS, AllowInsecure: allowInsecure,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("LDAP provider %q: %w", name, err)
		}
		ldapProviders[name] = provider
	}
	return oidcProviders, ldapProviders, nil
}
