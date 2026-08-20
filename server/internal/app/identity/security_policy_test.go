package identity

import (
	"errors"
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func securePolicyBaseline() domain.SecurityPolicy {
	return domain.SecurityPolicy{
		PasswordMinLength:        12,
		PasswordRequireUpper:     true,
		PasswordRequireLower:     true,
		PasswordRequireDigit:     true,
		PasswordRequireSymbol:    true,
		PasswordHistory:          5,
		PasswordMaxAgeDays:       90,
		LoginMaxFailures:         5,
		LoginLockDurationSeconds: 900,
		SessionTTLSeconds:        43200,
		MaxConcurrentSessions:    5,
	}
}

func TestValidateSecurityPolicyHardFloor(t *testing.T) {
	if err := validateSecurityPolicy(securePolicyBaseline()); err != nil {
		t.Fatalf("secure baseline rejected: %v", err)
	}
	tests := map[string]func(*domain.SecurityPolicy){
		"short password":          func(p *domain.SecurityPolicy) { p.PasswordMinLength = 11 },
		"upper disabled":          func(p *domain.SecurityPolicy) { p.PasswordRequireUpper = false },
		"lower disabled":          func(p *domain.SecurityPolicy) { p.PasswordRequireLower = false },
		"digit disabled":          func(p *domain.SecurityPolicy) { p.PasswordRequireDigit = false },
		"symbol disabled":         func(p *domain.SecurityPolicy) { p.PasswordRequireSymbol = false },
		"insufficient history":    func(p *domain.SecurityPolicy) { p.PasswordHistory = 4 },
		"expiration disabled":     func(p *domain.SecurityPolicy) { p.PasswordMaxAgeDays = 0 },
		"expiration too long":     func(p *domain.SecurityPolicy) { p.PasswordMaxAgeDays = 91 },
		"lockout disabled":        func(p *domain.SecurityPolicy) { p.LoginMaxFailures = 0 },
		"lockout too permissive":  func(p *domain.SecurityPolicy) { p.LoginMaxFailures = 6 },
		"lock duration too short": func(p *domain.SecurityPolicy) { p.LoginLockDurationSeconds = 899 },
		"session expiration off":  func(p *domain.SecurityPolicy) { p.SessionTTLSeconds = 0 },
		"session too long":        func(p *domain.SecurityPolicy) { p.SessionTTLSeconds = 43201 },
		"sessions unlimited":      func(p *domain.SecurityPolicy) { p.MaxConcurrentSessions = 0 },
		"too many sessions":       func(p *domain.SecurityPolicy) { p.MaxConcurrentSessions = 6 },
	}
	for name, weaken := range tests {
		t.Run(name, func(t *testing.T) {
			policy := securePolicyBaseline()
			weaken(&policy)
			if err := validateSecurityPolicy(policy); !errors.Is(err, ErrInvalidSecurityPolicy) {
				t.Fatalf("weak policy accepted, error = %v", err)
			}
		})
	}
}
