package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

var ErrInvalidAccessReview = errors.New("invalid access review")

func (s *Service) CreateAccessReview(ctx context.Context, actor domain.Principal, reviewerID, scopeType, scopeID string, dueAt time.Time) (domain.AccessReview, error) {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return domain.AccessReview{}, ErrInteractiveSessionRequired
	}
	if !actor.HasRole("system_admin", "security_admin", "auditor") || actor.UserID == strings.TrimSpace(reviewerID) {
		return domain.AccessReview{}, ErrGrantCeiling
	}
	if err := RequireRecentMFA(actor); err != nil {
		return domain.AccessReview{}, err
	}
	reviewerID = strings.TrimSpace(reviewerID)
	scopeType = strings.ToUpper(strings.TrimSpace(scopeType))
	scopeID = strings.TrimSpace(scopeID)
	if scopeType == "" {
		scopeType = domain.AccessReviewScopeAll
	}
	dueAt = dueAt.UTC()
	if reviewerID == "" || !validAccessReviewScope(scopeType, scopeID) || dueAt.Before(time.Now().UTC().Add(24*time.Hour)) || dueAt.After(time.Now().UTC().Add(90*24*time.Hour)) {
		return domain.AccessReview{}, ErrInvalidAccessReview
	}
	reviewer, err := s.repo.UserByID(ctx, reviewerID)
	if err != nil {
		return domain.AccessReview{}, err
	}
	if reviewer.OrganizationID != actor.OrganizationID || strings.ToUpper(reviewer.Status) != "ACTIVE" {
		return domain.AccessReview{}, ErrInvalidAccessReview
	}
	scopeName, err := s.accessReviewScopeName(ctx, actor.OrganizationID, scopeType, scopeID)
	if err != nil {
		return domain.AccessReview{}, ErrInvalidAccessReview
	}
	return s.repo.CreateAccessReview(ctx, domain.AccessReview{OrganizationID: actor.OrganizationID, ReviewerID: reviewerID, ScopeType: scopeType, ScopeID: scopeID, ScopeName: scopeName, Status: domain.AccessReviewOpen, DueAt: dueAt, CreatedBy: actor.UserID, CreatedAt: time.Now().UTC()})
}

func validAccessReviewScope(scopeType, scopeID string) bool {
	switch scopeType {
	case domain.AccessReviewScopeAll:
		return scopeID == ""
	case domain.AccessReviewScopeRole, domain.AccessReviewScopeDepartment, domain.AccessReviewScopeUser:
		return scopeID != ""
	default:
		return false
	}
}

func (s *Service) accessReviewScopeName(ctx context.Context, organizationID, scopeType, scopeID string) (string, error) {
	switch scopeType {
	case domain.AccessReviewScopeAll:
		return "全部用户", nil
	case domain.AccessReviewScopeUser:
		user, err := s.repo.UserByID(ctx, scopeID)
		if err != nil || user.OrganizationID != organizationID || strings.ToUpper(user.Status) != "ACTIVE" {
			return "", ErrInvalidAccessReview
		}
		if strings.TrimSpace(user.DisplayName) != "" {
			return user.DisplayName, nil
		}
		return user.LoginName, nil
	case domain.AccessReviewScopeDepartment:
		department, err := s.repo.DepartmentByID(ctx, organizationID, scopeID)
		if err != nil || strings.ToUpper(department.Status) != "ACTIVE" {
			return "", ErrInvalidAccessReview
		}
		return department.Name, nil
	case domain.AccessReviewScopeRole:
		roles, err := s.repo.ListRoles(ctx, organizationID)
		if err != nil {
			return "", err
		}
		for _, role := range roles {
			if role.Key == scopeID && strings.ToUpper(role.Status) == "ACTIVE" {
				return role.Name, nil
			}
		}
	}
	return "", ErrInvalidAccessReview
}

func (s *Service) ListAccessReviews(ctx context.Context, actor domain.Principal) ([]domain.AccessReview, error) {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return nil, ErrInteractiveSessionRequired
	}
	return s.repo.ListAccessReviews(ctx, actor.OrganizationID, 200)
}

func (s *Service) ListAccessReviewItems(ctx context.Context, actor domain.Principal, reviewID string) ([]domain.AccessReviewItem, error) {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return nil, ErrInteractiveSessionRequired
	}
	if strings.TrimSpace(reviewID) == "" {
		return nil, ErrInvalidAccessReview
	}
	review, err := s.repo.AccessReviewByID(ctx, actor.OrganizationID, reviewID)
	if err != nil {
		return nil, err
	}
	if review.ReviewerID != actor.UserID && !actor.HasRole("system_admin", "security_admin", "auditor") {
		return nil, ErrGrantCeiling
	}
	return s.repo.ListAccessReviewItems(ctx, actor.OrganizationID, reviewID)
}

func (s *Service) DecideAccessReviewItem(ctx context.Context, actor domain.Principal, reviewID, itemID, decision, reason string) error {
	if actor.UserID == "" || actor.OrganizationID == "" {
		return ErrInteractiveSessionRequired
	}
	if err := RequireRecentMFA(actor); err != nil {
		return err
	}
	decision = strings.ToUpper(strings.TrimSpace(decision))
	reason = strings.TrimSpace(reason)
	if reviewID == "" || itemID == "" || (decision != domain.AccessReviewApprove && decision != domain.AccessReviewRevoke && decision != domain.AccessReviewException) || ((decision == domain.AccessReviewRevoke || decision == domain.AccessReviewException) && len(reason) < 8) || len(reason) > 500 {
		return ErrInvalidAccessReview
	}
	return s.repo.DecideAccessReviewItem(ctx, actor.OrganizationID, reviewID, itemID, actor.UserID, decision, reason)
}
