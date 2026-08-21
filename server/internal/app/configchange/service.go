package configchange

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	identitydomain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	platformconfig "github.com/sevoniva-labs/velora/server/internal/platform/configchange"
)

var (
	ErrOrganizationRequired = errors.New("organization is required")
	ErrActorRequired        = errors.New("interactive actor is required")
)

type Repository interface {
	List(context.Context, string) ([]platformconfig.Change, error)
	ByID(context.Context, string, string) (platformconfig.Change, error)
	Create(context.Context, platformconfig.Change) (platformconfig.Change, error)
	Update(context.Context, platformconfig.Change) (platformconfig.Change, error)
}

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

type CreateInput struct {
	Namespace               string
	Group                   string
	DataID                  string
	Version                 uint64
	ExpectedPreviousVersion uint64
	ValueDigest             string
	ValueRef                string
	Sensitive               bool
}

func (s *Service) List(ctx context.Context, actor identitydomain.Principal) ([]platformconfig.Change, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, actor.OrganizationID)
}

func (s *Service) Create(ctx context.Context, actor identitydomain.Principal, input CreateInput) (platformconfig.Change, error) {
	if err := requireActor(actor); err != nil {
		return platformconfig.Change{}, err
	}
	change, err := platformconfig.New(uuid.NewString(), actor.OrganizationID, input.Namespace, input.Group, input.DataID, input.ValueDigest, input.ValueRef, actor.UserID, input.Version, input.ExpectedPreviousVersion, input.Sensitive, time.Now().UTC())
	if err != nil {
		return platformconfig.Change{}, err
	}
	return s.repo.Create(ctx, change)
}

func (s *Service) Transition(ctx context.Context, actor identitydomain.Principal, id string, request platformconfig.Request) (platformconfig.Change, platformconfig.AuditEntry, error) {
	if err := requireActor(actor); err != nil {
		return platformconfig.Change{}, platformconfig.AuditEntry{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return platformconfig.Change{}, platformconfig.AuditEntry{}, errors.New("config change id is required")
	}
	change, err := s.repo.ByID(ctx, actor.OrganizationID, id)
	if err != nil {
		return platformconfig.Change{}, platformconfig.AuditEntry{}, err
	}
	if request.ActorID == "" {
		request.ActorID = actor.UserID
	}
	next, entry, err := platformconfig.Apply(change, request)
	if err != nil {
		return platformconfig.Change{}, platformconfig.AuditEntry{}, err
	}
	saved, err := s.repo.Update(ctx, next)
	if err != nil {
		return platformconfig.Change{}, platformconfig.AuditEntry{}, err
	}
	return saved, entry, nil
}

func requireActor(actor identitydomain.Principal) error {
	if actor.Type != "USER" || strings.TrimSpace(actor.UserID) == "" || strings.TrimSpace(actor.OrganizationID) == "" {
		return ErrActorRequired
	}
	return nil
}
