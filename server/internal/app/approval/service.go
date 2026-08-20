package approval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	identityapp "github.com/sevoniva-labs/velora/server/internal/app/identity"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/approval"
	identitydomain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

type Service struct{ repo *repository.ApprovalRepo }

var (
	ErrAccessDenied     = errors.New("approval access denied")
	ErrApprovalRequired = errors.New("approval execution ticket is required")
)

func NewService(repo *repository.ApprovalRepo) *Service { return &Service{repo: repo} }

type CreateInput struct {
	RequestType       string
	Action            string
	Resource          string
	ResourceID        string
	Summary           string
	PayloadJSON       string
	Mode              string
	RequiredApprovals int
	ApproverIDs       []string
	ExpiresIn         time.Duration
}

type ExecutionInput struct {
	RequestType string
	Action      string
	Resource    string
	ResourceID  string
	PayloadJSON string
}

func (s *Service) Create(ctx context.Context, actor identitydomain.Principal, input CreateInput) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return domain.Request{}, err
	}
	input.RequestType = strings.TrimSpace(input.RequestType)
	input.Action = strings.TrimSpace(input.Action)
	input.Resource = strings.TrimSpace(input.Resource)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Mode = strings.ToUpper(strings.TrimSpace(input.Mode))
	if len(input.PayloadJSON) == 0 || len(input.PayloadJSON) > 16*1024 || len(input.RequestType) > 100 || len(input.Action) > 160 || len(input.Resource) > 160 || len(input.ResourceID) > 160 || len(input.Summary) > 500 || input.ExpiresIn < time.Minute || input.ExpiresIn > 30*24*time.Hour {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	canonical, err := canonicalJSON([]byte(input.PayloadJSON))
	if err != nil || validateReviewablePayload(canonical) != nil {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	digest := requestDigest(input.RequestType, input.Action, input.Resource, input.ResourceID, canonical)
	now := time.Now().UTC()
	request := domain.Request{ID: uuid.NewString(), OrganizationID: actor.OrganizationID, RequestType: input.RequestType, Action: input.Action, Resource: input.Resource, ResourceID: input.ResourceID, Summary: input.Summary, PayloadJSON: string(canonical), RequestDigest: digest, ApplicantID: actor.UserID, Mode: input.Mode, RequiredApprovals: input.RequiredApprovals, Status: domain.StatusPending, ExpiresAt: now.Add(input.ExpiresIn), CreatedAt: now, UpdatedAt: now}
	if err := domain.ValidateCreation(request, input.ApproverIDs); err != nil {
		return domain.Request{}, err
	}
	return s.repo.Create(ctx, request, input.ApproverIDs)
}

func validateReviewablePayload(canonical []byte) error {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil || payload == nil {
		return domain.ErrInvalidRequest
	}
	blocked := map[string]struct{}{
		"password": {}, "passwd": {}, "secret": {}, "clientsecret": {}, "accesstoken": {}, "refreshtoken": {},
		"credential": {}, "credentials": {}, "privatekey": {}, "recoverycode": {}, "mfacode": {},
		"authorization": {}, "cookie": {}, "sessiontoken": {},
	}
	var inspect func(any) error
	inspect = func(value any) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				normalized := strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(strings.ToLower(key))
				if _, denied := blocked[normalized]; denied {
					return domain.ErrInvalidRequest
				}
				if err := inspect(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := inspect(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return inspect(payload)
}

func (s *Service) AuthorizeExecution(ctx context.Context, actor identitydomain.Principal, approvalID string, input ExecutionInput) error {
	if strings.TrimSpace(approvalID) == "" {
		return ErrApprovalRequired
	}
	if err := requireActor(actor); err != nil {
		return err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return err
	}
	canonical, err := canonicalJSON([]byte(input.PayloadJSON))
	if err != nil {
		return domain.ErrInvalidRequest
	}
	digest := requestDigest(strings.TrimSpace(input.RequestType), strings.TrimSpace(input.Action), strings.TrimSpace(input.Resource), strings.TrimSpace(input.ResourceID), canonical)
	return s.repo.ClaimExecution(ctx, actor.OrganizationID, strings.TrimSpace(approvalID), actor.UserID, digest)
}

func (s *Service) Get(ctx context.Context, actor identitydomain.Principal, requestID string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	request, err := s.repo.ByID(ctx, actor.OrganizationID, strings.TrimSpace(requestID))
	if err != nil {
		return domain.Request{}, err
	}
	if !repository.IsApprovalParticipant(request, actor.UserID) && !actor.HasRole("system_admin", "auditor") {
		return domain.Request{}, ErrAccessDenied
	}
	return request, nil
}

func (s *Service) List(ctx context.Context, actor identitydomain.Principal) ([]domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, actor.OrganizationID, actor.UserID, actor.HasRole("system_admin", "auditor"), 200)
}

func (s *Service) Decide(ctx context.Context, actor identitydomain.Principal, requestID, decision, comment string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return domain.Request{}, err
	}
	decision = strings.ToUpper(strings.TrimSpace(decision))
	if (decision != domain.DecisionApprove && decision != domain.DecisionReject) || len(comment) > 1000 {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	return s.repo.Decide(ctx, actor.OrganizationID, strings.TrimSpace(requestID), actor.UserID, decision, strings.TrimSpace(comment))
}

func (s *Service) Transfer(ctx context.Context, actor identitydomain.Principal, requestID, newAssigneeID, comment string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return domain.Request{}, err
	}
	if newAssigneeID = strings.TrimSpace(newAssigneeID); newAssigneeID == "" || len(comment) > 1000 {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	return s.repo.Transfer(ctx, actor.OrganizationID, strings.TrimSpace(requestID), actor.UserID, newAssigneeID, strings.TrimSpace(comment))
}

func (s *Service) Withdraw(ctx context.Context, actor identitydomain.Principal, requestID, comment string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if len(comment) > 1000 {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	return s.repo.Withdraw(ctx, actor.OrganizationID, strings.TrimSpace(requestID), actor.UserID, strings.TrimSpace(comment))
}

func requireActor(actor identitydomain.Principal) error {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" {
		return identityapp.ErrInteractiveSessionRequired
	}
	return nil
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, domain.ErrInvalidRequest
		}
		return nil, err
	}
	return json.Marshal(value)
}

func requestDigest(requestType, action, resource, resourceID string, canonicalPayload []byte) string {
	digestInput, _ := json.Marshal([]any{requestType, action, resource, resourceID, canonicalPayload})
	digest := sha256.Sum256(digestInput)
	return hex.EncodeToString(digest[:])
}
