package kratosapi

import (
	"context"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func (s *PlatformService) ListAccessReviews(ctx context.Context, _ *forgev1.ListAccessReviewsRequest) (*forgev1.ListAccessReviewsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	reviews, err := s.identity.ListAccessReviews(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	response := &forgev1.ListAccessReviewsResponse{Reviews: make([]*forgev1.AccessReview, 0, len(reviews))}
	for _, review := range reviews {
		response.Reviews = append(response.Reviews, accessReviewProto(review))
	}
	return response, nil
}

func (s *PlatformService) CreateAccessReview(ctx context.Context, req *forgev1.CreateAccessReviewRequest) (*forgev1.CreateAccessReviewResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetDueAt() == nil {
		return nil, serviceError(appidentity.ErrInvalidAccessReview)
	}
	event := newAuditEvent(ctx, principal, "access_review.create", "access_review", "", map[string]any{"reviewer_id": req.GetReviewerId(), "due_at": req.GetDueAt().AsTime(), "scope_type": req.GetScopeType(), "scope_id": req.GetScopeId()})
	var review domain.AccessReview
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		review, createErr = s.identity.CreateAccessReview(txCtx, principal, req.GetReviewerId(), req.GetScopeType(), req.GetScopeId(), req.GetDueAt().AsTime())
		if createErr == nil {
			event.ResourceID = review.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateAccessReviewResponse{Review: accessReviewProto(review)}, nil
}

func (s *PlatformService) ListAccessReviewItems(ctx context.Context, req *forgev1.ListAccessReviewItemsRequest) (*forgev1.ListAccessReviewItemsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.identity.ListAccessReviewItems(ctx, principal, req.GetReviewId())
	if err != nil {
		return nil, serviceError(err)
	}
	response := &forgev1.ListAccessReviewItemsResponse{Items: make([]*forgev1.AccessReviewItem, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, accessReviewItemProto(item))
	}
	return response, nil
}

func (s *PlatformService) DecideAccessReviewItem(ctx context.Context, req *forgev1.DecideAccessReviewItemRequest) (*forgev1.DecideAccessReviewItemResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "access_review.item.decide", "access_review_item", req.GetItemId(), map[string]any{"review_id": req.GetReviewId(), "decision": req.GetDecision()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.identity.DecideAccessReviewItem(txCtx, principal, req.GetReviewId(), req.GetItemId(), req.GetDecision(), req.GetReason()); err != nil {
			return err
		}
		if req.GetDecision() == domain.AccessReviewRevoke {
			return s.recomputeApplicationAccess(txCtx, principal)
		}
		return nil
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.DecideAccessReviewItemResponse{}, nil
}

func accessReviewProto(review domain.AccessReview) *forgev1.AccessReview {
	return &forgev1.AccessReview{Id: review.ID, OrganizationId: review.OrganizationID, ReviewerId: review.ReviewerID, ReviewerName: review.ReviewerName, Status: review.Status, DueAt: timestamp(review.DueAt), CreatedBy: review.CreatedBy, CreatedAt: timestamp(review.CreatedAt), CompletedAt: optionalTimestamp(review.CompletedAt), ScopeType: review.ScopeType, ScopeId: review.ScopeID, ScopeName: review.ScopeName, ItemCount: boundedInt32(review.ItemCount), PendingCount: boundedInt32(review.PendingCount)}
}

func boundedInt32(value int) int32 {
	if value <= 0 {
		return 0
	}
	if value > 2147483647 {
		return 2147483647
	}
	return int32(value) // #nosec G115 -- value is explicitly bounded to the int32 range above.
}

func accessReviewItemProto(item domain.AccessReviewItem) *forgev1.AccessReviewItem {
	return &forgev1.AccessReviewItem{Id: item.ID, ReviewId: item.ReviewID, OrganizationId: item.OrganizationID, UserId: item.UserID, LoginName: item.LoginName, RoleKey: item.RoleKey, Decision: item.Decision, Reason: item.Reason, DecidedBy: item.DecidedBy, DecidedAt: optionalTimestamp(item.DecidedAt), CreatedAt: timestamp(item.CreatedAt)}
}
