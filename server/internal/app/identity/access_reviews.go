package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

var ErrInvalidAccessReview = errors.New("invalid access review")

func (s *Service) CreateAccessReview(ctx context.Context, actor domain.Principal, reviewerID string, dueAt time.Time) (domain.AccessReview, error) {
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
	dueAt = dueAt.UTC()
	if reviewerID == "" || dueAt.Before(time.Now().UTC().Add(24*time.Hour)) || dueAt.After(time.Now().UTC().Add(90*24*time.Hour)) {
		return domain.AccessReview{}, ErrInvalidAccessReview
	}
	reviewer, err := s.repo.UserByID(ctx, reviewerID)
	if err != nil {
		return domain.AccessReview{}, err
	}
	if reviewer.OrganizationID != actor.OrganizationID || strings.ToUpper(reviewer.Status) != "ACTIVE" {
		return domain.AccessReview{}, ErrInvalidAccessReview
	}
	return s.repo.CreateAccessReview(ctx, domain.AccessReview{OrganizationID: actor.OrganizationID, ReviewerID: reviewerID, Status: domain.AccessReviewOpen, DueAt: dueAt, CreatedBy: actor.UserID, CreatedAt: time.Now().UTC()})
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
