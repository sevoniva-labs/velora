package identity

import "context"

// ManagedIdentityProvider is the narrow application port for the credential
// authority. It intentionally excludes Casdoor applications, roles, MFA and
// sessions: Velora only needs account lifecycle operations.
type ManagedIdentityProvider interface {
	Enabled() bool
	CreateUser(context.Context, ManagedUserInput) (string, error)
	SetUserStatus(context.Context, string, bool) error
	SetUserPassword(context.Context, string, string) error
}

type ManagedUserInput struct {
	LoginName   string
	DisplayName string
	Email       string
	Password    string
}

func (s *Service) ConfigureManagedIdentityProvider(provider ManagedIdentityProvider, issuer string) {
	s.managedIdentity = provider
	s.identityIssuer = issuer
}

func (s *Service) ManagedIdentityEnabled() bool {
	return s.managedIdentity != nil && s.managedIdentity.Enabled()
}
