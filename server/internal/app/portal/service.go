package portal

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"

	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

var (
	ErrNotFound     = errors.New("portal resource not found")
	ErrAccessDenied = errors.New("portal access denied")
	ErrDisabled     = errors.New("portal application is disabled")
	ErrInvalid      = errors.New("invalid portal request")
)

type Service struct{ repo *repository.PortalRepo }

func NewService(repo *repository.PortalRepo) *Service { return &Service{repo: repo} }

func (s *Service) ListApplications(ctx context.Context, principal domain.Principal, filter repository.ApplicationFilter) ([]portaldomain.Application, error) {
	items, err := s.repo.ListApplications(ctx, principal.OrganizationID, principal.UserID, filter, false)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroupKeys(ctx, principal.OrganizationID, principal.UserID)
	if err != nil {
		return nil, err
	}
	access := portaldomain.AccessContext{Principal: principal, Groups: groups}
	out := make([]portaldomain.Application, 0, len(items))
	for _, item := range items {
		if portaldomain.CanAccess(item, access) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Service) GetApplication(ctx context.Context, principal domain.Principal, id string) (portaldomain.Application, error) {
	item, err := s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), false)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Application{}, ErrNotFound
	}
	if err != nil {
		return portaldomain.Application{}, err
	}
	groups, err := s.repo.ListGroupKeys(ctx, principal.OrganizationID, principal.UserID)
	if err != nil {
		return portaldomain.Application{}, err
	}
	if !portaldomain.CanAccess(item, portaldomain.AccessContext{Principal: principal, Groups: groups}) {
		return portaldomain.Application{}, ErrAccessDenied
	}
	return item, nil
}

func (s *Service) Launch(ctx context.Context, principal domain.Principal, id string) (portaldomain.Application, string, error) {
	item, err := s.GetApplication(ctx, principal, id)
	if err != nil {
		return portaldomain.Application{}, "", err
	}
	launchURL := strings.TrimSpace(item.LaunchURL)
	if launchURL == "" {
		launchURL = strings.TrimSpace(item.HomeURL)
	}
	u, err := url.Parse(launchURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return portaldomain.Application{}, "", portaldomain.ErrInvalidLaunchURL
	}
	if err := s.repo.RecordVisit(ctx, principal.OrganizationID, principal.UserID, item.ID); err != nil {
		return portaldomain.Application{}, "", err
	}
	item.VisitCount++
	return item, launchURL, nil
}

func (s *Service) ListFavorites(ctx context.Context, principal domain.Principal, limit int) ([]portaldomain.Application, error) {
	items, err := s.repo.ListFavorites(ctx, principal.OrganizationID, principal.UserID, limit)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroupKeys(ctx, principal.OrganizationID, principal.UserID)
	if err != nil {
		return nil, err
	}
	access := portaldomain.AccessContext{Principal: principal, Groups: groups}
	out := make([]portaldomain.Application, 0, len(items))
	for _, item := range items {
		if portaldomain.CanAccess(item, access) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Service) AddFavorite(ctx context.Context, principal domain.Principal, id string) (portaldomain.Application, error) {
	item, err := s.GetApplication(ctx, principal, id)
	if err != nil {
		return portaldomain.Application{}, err
	}
	if err := s.repo.AddFavorite(ctx, principal.OrganizationID, principal.UserID, item.ID); err != nil {
		return portaldomain.Application{}, err
	}
	item.Favorite = true
	return item, nil
}

func (s *Service) RemoveFavorite(ctx context.Context, principal domain.Principal, id string) error {
	item, err := s.GetApplication(ctx, principal, id)
	if err != nil {
		return err
	}
	return s.repo.RemoveFavorite(ctx, principal.OrganizationID, principal.UserID, item.ID)
}

func (s *Service) ListRecent(ctx context.Context, principal domain.Principal, limit int) ([]portaldomain.Application, error) {
	items, err := s.repo.ListRecent(ctx, principal.OrganizationID, principal.UserID, limit)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroupKeys(ctx, principal.OrganizationID, principal.UserID)
	if err != nil {
		return nil, err
	}
	access := portaldomain.AccessContext{Principal: principal, Groups: groups}
	out := make([]portaldomain.Application, 0, len(items))
	for _, item := range items {
		if portaldomain.CanAccess(item, access) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Service) AdminListApplications(ctx context.Context, principal domain.Principal, limit int) ([]portaldomain.Application, error) {
	return s.repo.ListApplications(ctx, principal.OrganizationID, principal.UserID, repository.ApplicationFilter{Limit: limit}, true)
}

func (s *Service) CreateApplication(ctx context.Context, principal domain.Principal, input repository.ApplicationInput) (portaldomain.Application, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = portaldomain.StatusEnabled
	}
	if input.LaunchType == "" {
		input.LaunchType = "URL"
	}
	if err := portaldomain.ValidateApplication(portaldomain.Application{Code: input.Code, Name: input.Name, LaunchURL: input.LaunchURL, Status: input.Status}); err != nil {
		return portaldomain.Application{}, err
	}
	return s.repo.CreateApplication(ctx, principal.OrganizationID, principal.UserID, input)
}

func (s *Service) UpdateApplication(ctx context.Context, principal domain.Principal, id string, input repository.ApplicationInput) (portaldomain.Application, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = portaldomain.StatusEnabled
	}
	if input.LaunchType == "" {
		input.LaunchType = "URL"
	}
	if err := portaldomain.ValidateApplication(portaldomain.Application{Code: "valid", Name: input.Name, LaunchURL: input.LaunchURL, Status: input.Status}); err != nil {
		return portaldomain.Application{}, err
	}
	item, err := s.repo.UpdateApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), input)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Application{}, ErrNotFound
	}
	return item, err
}

func (s *Service) DeleteApplication(ctx context.Context, principal domain.Principal, id string) error {
	err := s.repo.DeleteApplication(ctx, principal.OrganizationID, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) Categories(ctx context.Context, orgID string) ([]portaldomain.Category, error) {
	return s.repo.ListCategories(ctx, orgID)
}
func (s *Service) Tags(ctx context.Context, orgID string) ([]portaldomain.Tag, error) {
	return s.repo.ListTags(ctx, orgID)
}

func (s *Service) CreateCategory(ctx context.Context, principal domain.Principal, input repository.CategoryInput) (portaldomain.Category, error) {
	input.Key, input.Name = strings.TrimSpace(input.Key), strings.TrimSpace(input.Name)
	if input.Key == "" || input.Name == "" {
		return portaldomain.Category{}, ErrInvalid
	}
	if input.Status == "" {
		input.Status = portaldomain.StatusActive
	}
	return s.repo.CreateCategory(ctx, principal.OrganizationID, input)
}
func (s *Service) UpdateCategory(ctx context.Context, principal domain.Principal, id string, input repository.CategoryInput) (portaldomain.Category, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return portaldomain.Category{}, ErrInvalid
	}
	if input.Status == "" {
		input.Status = portaldomain.StatusActive
	}
	item, err := s.repo.UpdateCategory(ctx, principal.OrganizationID, id, input)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Category{}, ErrNotFound
	}
	return item, err
}
func (s *Service) DeleteCategory(ctx context.Context, principal domain.Principal, id string) error {
	err := s.repo.DeleteCategory(ctx, principal.OrganizationID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) CreateTag(ctx context.Context, principal domain.Principal, input repository.TagInput) (portaldomain.Tag, error) {
	input.Key, input.Name = strings.TrimSpace(input.Key), strings.TrimSpace(input.Name)
	if input.Key == "" || input.Name == "" {
		return portaldomain.Tag{}, ErrInvalid
	}
	return s.repo.CreateTag(ctx, principal.OrganizationID, input)
}
func (s *Service) UpdateTag(ctx context.Context, principal domain.Principal, id string, input repository.TagInput) (portaldomain.Tag, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return portaldomain.Tag{}, ErrInvalid
	}
	item, err := s.repo.UpdateTag(ctx, principal.OrganizationID, id, input)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Tag{}, ErrNotFound
	}
	return item, err
}
func (s *Service) DeleteTag(ctx context.Context, principal domain.Principal, id string) error {
	err := s.repo.DeleteTag(ctx, principal.OrganizationID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) ReplacePolicies(ctx context.Context, principal domain.Principal, appID string, policies []portaldomain.AccessPolicy) ([]portaldomain.AccessPolicy, error) {
	items, err := s.repo.ReplacePolicies(ctx, principal.OrganizationID, strings.TrimSpace(appID), policies)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return items, err
}
