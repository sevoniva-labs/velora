package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

var ErrInvalidTemporaryGrant = errors.New("invalid temporary role grant")

func (s *Service) CreateTemporaryRoleGrant(ctx context.Context, actor domain.Principal, userID, roleKey, reason, approvalID string, validFrom, validUntil time.Time) (domain.TemporaryRoleGrant, error) {
	if err := authorizeGrantActor(actor, actor.OrganizationID); err != nil {
		return domain.TemporaryRoleGrant{}, err
	}
	if actor.UserID == userID {
		return domain.TemporaryRoleGrant{}, ErrGrantCeiling
	}
	roleKey = strings.TrimSpace(roleKey)
	reason = strings.TrimSpace(reason)
	approvalID = strings.TrimSpace(approvalID)
	validFrom = validFrom.UTC()
	validUntil = validUntil.UTC()
	if roleKey == "" || roleKey == "system_admin" || len(reason) < 8 || len(reason) > 500 || approvalID == "" || validFrom.IsZero() || !validUntil.After(validFrom) || !validUntil.After(time.Now().UTC()) {
		return domain.TemporaryRoleGrant{}, ErrInvalidTemporaryGrant
	}
	maxDuration := 30 * 24 * time.Hour
	if roleKey == "security_admin" || roleKey == "auditor" {
		maxDuration = 8 * time.Hour
	}
	if validUntil.Sub(validFrom) > maxDuration {
		return domain.TemporaryRoleGrant{}, ErrInvalidTemporaryGrant
	}
	target, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return domain.TemporaryRoleGrant{}, err
	}
	if target.OrganizationID != actor.OrganizationID || target.Status != "ACTIVE" {
		return domain.TemporaryRoleGrant{}, ErrGrantCeiling
	}
	roles, err := s.repo.ListRoles(ctx, actor.OrganizationID)
	if err != nil {
		return domain.TemporaryRoleGrant{}, err
	}
	allowed := false
	for _, role := range roles {
		if role.Key == roleKey {
			allowed = true
			break
		}
	}
	if !allowed {
		return domain.TemporaryRoleGrant{}, ErrInvalidRole
	}
	nextRoles := append(append([]string(nil), target.Roles...), roleKey)
	if err := enforceRoleMutation(actor, target.Roles, nextRoles); err != nil {
		return domain.TemporaryRoleGrant{}, err
	}
	if err := s.validateRoleCombination(ctx, actor.OrganizationID, nextRoles); err != nil {
		return domain.TemporaryRoleGrant{}, err
	}
	return s.repo.CreateTemporaryRoleGrant(ctx, domain.TemporaryRoleGrant{
		OrganizationID: actor.OrganizationID,
		UserID:         userID,
		RoleKey:        roleKey,
		RequestedBy:    actor.UserID,
		ApprovalID:     approvalID,
		Reason:         reason,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
	})
}

func (s *Service) ListTemporaryRoleGrants(ctx context.Context, actor domain.Principal) ([]domain.TemporaryRoleGrant, error) {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return nil, ErrInteractiveSessionRequired
	}
	return s.repo.ListTemporaryRoleGrants(ctx, actor.OrganizationID)
}

func (s *Service) RevokeTemporaryRoleGrant(ctx context.Context, actor domain.Principal, grantID, reason string) error {
	if err := authorizeGrantActor(actor, actor.OrganizationID); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if strings.TrimSpace(grantID) == "" || len(reason) < 4 || len(reason) > 500 {
		return ErrInvalidTemporaryGrant
	}
	return s.repo.RevokeTemporaryRoleGrant(ctx, actor.OrganizationID, grantID, actor.UserID, reason)
}
