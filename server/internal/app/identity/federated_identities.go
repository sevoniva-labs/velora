package identity

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

var ErrInvalidFederatedIdentity = errors.New("invalid federated identity mapping")

var federatedProviderPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func (s *Service) ListFederatedIdentityLinks(ctx context.Context, actor domain.Principal) ([]domain.FederatedIdentityLink, error) {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return nil, ErrInteractiveSessionRequired
	}
	return s.repo.ListFederatedIdentityLinks(ctx, actor.OrganizationID)
}

func (s *Service) LinkFederatedIdentity(ctx context.Context, actor domain.Principal, provider, subject, userID, approvalID string) (domain.FederatedIdentityLink, error) {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return domain.FederatedIdentityLink{}, ErrInteractiveSessionRequired
	}
	if err := RequireRecentMFA(actor); err != nil {
		return domain.FederatedIdentityLink{}, err
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	userID = strings.TrimSpace(userID)
	approvalID = strings.TrimSpace(approvalID)
	if !federatedProviderPattern.MatchString(provider) || subject == "" || len(subject) > 512 || userID == "" || approvalID == "" || userID == actor.UserID {
		return domain.FederatedIdentityLink{}, ErrInvalidFederatedIdentity
	}
	target, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return domain.FederatedIdentityLink{}, err
	}
	if target.OrganizationID != actor.OrganizationID || strings.ToUpper(target.Status) != "ACTIVE" {
		return domain.FederatedIdentityLink{}, ErrInvalidFederatedIdentity
	}
	return s.repo.CreateFederatedIdentityLink(ctx, domain.FederatedIdentityLink{
		OrganizationID: actor.OrganizationID,
		Provider:       provider,
		Subject:        subject,
		UserID:         target.ID,
		CreatedBy:      actor.UserID,
		ApprovalID:     approvalID,
		CreatedAt:      time.Now().UTC(),
	})
}

func (s *Service) UnlinkFederatedIdentity(ctx context.Context, actor domain.Principal, linkID string) error {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return ErrInteractiveSessionRequired
	}
	if err := RequireRecentMFA(actor); err != nil {
		return err
	}
	if strings.TrimSpace(linkID) == "" {
		return ErrInvalidFederatedIdentity
	}
	return s.repo.DeleteFederatedIdentityLink(ctx, actor.OrganizationID, strings.TrimSpace(linkID))
}
