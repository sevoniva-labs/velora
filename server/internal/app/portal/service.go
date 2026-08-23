package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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

type Service struct {
	repo              *repository.PortalRepo
	allowedOIDCIssuer string
}

func NewService(repo *repository.PortalRepo) *Service { return &Service{repo: repo} }

// ConfigureOIDCIssuer fixes application onboarding to the platform's trusted
// Casdoor issuer. Supporting arbitrary federation issuers requires a separate
// allowlist and hardened egress design and is intentionally not implicit.
func (s *Service) ConfigureOIDCIssuer(issuer string) {
	s.allowedOIDCIssuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
}

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
	if strings.EqualFold(strings.TrimSpace(item.LaunchType), portaldomain.LaunchTypeRetiredVeloraOIDC) {
		return portaldomain.Application{}, "", portaldomain.ErrRetiredLaunchType
	}
	launchURL := strings.TrimSpace(item.LaunchURL)
	if launchURL == "" {
		launchURL = strings.TrimSpace(item.HomeURL)
	}
	u, err := url.Parse(launchURL)
	if err != nil || u.Host == "" || u.Scheme != "https" {
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

func (s *Service) GetApplicationOnboarding(ctx context.Context, principal domain.Principal, id string) (portaldomain.Application, portaldomain.IdentityBinding, []portaldomain.Verification, error) {
	app, err := s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Application{}, portaldomain.IdentityBinding{}, nil, ErrNotFound
	}
	if err != nil {
		return portaldomain.Application{}, portaldomain.IdentityBinding{}, nil, err
	}
	binding, bindingErr := s.repo.GetIdentityBinding(ctx, principal.OrganizationID, app.ID)
	if bindingErr != nil && !errors.Is(bindingErr, sql.ErrNoRows) {
		return portaldomain.Application{}, portaldomain.IdentityBinding{}, nil, bindingErr
	}
	verifications, err := s.repo.ListIdentityVerifications(ctx, principal.OrganizationID, app.ID, 50)
	if err != nil {
		return portaldomain.Application{}, portaldomain.IdentityBinding{}, nil, err
	}
	return app, binding, verifications, nil
}

func (s *Service) UpsertApplicationIdentityBinding(ctx context.Context, principal domain.Principal, id string, input portaldomain.IdentityBindingInput, expectedVersion int64) (portaldomain.IdentityBinding, portaldomain.Application, error) {
	if err := input.Validate(); err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, err
	}
	if strings.EqualFold(strings.TrimSpace(input.Protocol), portaldomain.ProtocolOIDC) &&
		(s.allowedOIDCIssuer == "" || strings.TrimRight(strings.TrimSpace(input.Issuer), "/") != s.allowedOIDCIssuer) {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, portaldomain.ErrInvalidIdentityBinding
	}
	app, err := s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, ErrNotFound
	}
	if err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, err
	}
	binding, err := s.repo.UpsertIdentityBinding(ctx, principal.OrganizationID, principal.UserID, app.ID, input, expectedVersion)
	if err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, err
	}
	app, err = s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, app.ID, true)
	return binding, app, err
}

func (s *Service) VerifyApplicationIdentity(ctx context.Context, principal domain.Principal, id string, expectedVersion int64) (portaldomain.IdentityBinding, portaldomain.Application, []portaldomain.Verification, bool, error) {
	app, binding, _, err := s.GetApplicationOnboarding(ctx, principal, id)
	if err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, nil, false, err
	}
	if binding.ID == "" {
		return portaldomain.IdentityBinding{}, app, nil, false, portaldomain.ErrIdentityBindingRequired
	}
	passed, checkType, errorCode, evidence := verifyBinding(ctx, binding, s.allowedOIDCIssuer)
	binding, _, err = s.repo.RecordIdentityVerification(ctx, principal.OrganizationID, principal.UserID, app.ID, passed, checkType, errorCode, evidence, requestID(ctx), expectedVersion)
	if err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, nil, false, err
	}
	app, err = s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, app.ID, true)
	if err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Application{}, nil, false, err
	}
	verifications, err := s.repo.ListIdentityVerifications(ctx, principal.OrganizationID, app.ID, 50)
	return binding, app, verifications, passed, err
}

func (s *Service) PublishApplication(ctx context.Context, principal domain.Principal, id string, expectedVersion int64) (portaldomain.Application, error) {
	app, binding, _, err := s.GetApplicationOnboarding(ctx, principal, id)
	if err != nil {
		return portaldomain.Application{}, err
	}
	if strings.EqualFold(app.LaunchType, "URL") {
		// URL applications have no identity-side dependency.
	} else if binding.ID == "" || binding.VerificationStatus != portaldomain.VerificationPassed || app.LifecycleStatus != portaldomain.LifecycleReady {
		return portaldomain.Application{}, portaldomain.ErrPublishNotReady
	}
	return s.repo.SetApplicationLifecycle(ctx, principal.OrganizationID, principal.UserID, app.ID, portaldomain.LifecyclePublished, portaldomain.StatusEnabled, expectedVersion, true)
}

func (s *Service) SubmitApplicationPublish(ctx context.Context, principal domain.Principal, id string, expectedVersion int64) (portaldomain.Application, error) {
	app, binding, _, err := s.GetApplicationOnboarding(ctx, principal, id)
	if err != nil {
		return portaldomain.Application{}, err
	}
	if !strings.EqualFold(app.LaunchType, "URL") && (binding.ID == "" || binding.VerificationStatus != portaldomain.VerificationPassed) {
		return portaldomain.Application{}, portaldomain.ErrPublishNotReady
	}
	return s.repo.SetApplicationLifecycle(ctx, principal.OrganizationID, principal.UserID, app.ID, portaldomain.LifecycleReady, portaldomain.StatusDisabled, expectedVersion, false)
}

func (s *Service) DisableApplication(ctx context.Context, principal domain.Principal, id string, expectedVersion int64) (portaldomain.Application, error) {
	return s.repo.SetApplicationLifecycle(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), portaldomain.LifecycleDisabled, portaldomain.StatusDisabled, expectedVersion, false)
}

func verifyBinding(ctx context.Context, binding portaldomain.IdentityBinding, allowedIssuer string) (bool, string, string, string) {
	if strings.EqualFold(binding.Protocol, portaldomain.ProtocolOIDC) {
		issuer := strings.TrimRight(strings.TrimSpace(binding.Issuer), "/")
		allowedIssuer = strings.TrimRight(strings.TrimSpace(allowedIssuer), "/")
		if issuer == "" || allowedIssuer == "" {
			return false, "oidc_discovery", "ISSUER_REQUIRED", `{"reason":"issuer is required"}`
		}
		if issuer != allowedIssuer {
			return false, "oidc_discovery", "ISSUER_NOT_ALLOWED", `{"reason":"issuer is not allowed"}`
		}
		u, err := url.Parse(issuer + "/.well-known/openid-configuration")
		if err != nil {
			return false, "oidc_discovery", "ISSUER_INVALID", `{"reason":"issuer is invalid"}`
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return false, "oidc_discovery", "DISCOVERY_REQUEST_INVALID", `{"reason":"discovery request invalid"}`
		}
		client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Do(req)
		if err != nil {
			return false, "oidc_discovery", "DISCOVERY_UNREACHABLE", `{"reason":"discovery endpoint unavailable"}`
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return false, "oidc_discovery", fmt.Sprintf("DISCOVERY_HTTP_%d", resp.StatusCode), `{"reason":"discovery endpoint returned non-200"}`
		}
		var payload struct {
			Issuer string `json:"issuer"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil || strings.TrimRight(payload.Issuer, "/") != issuer {
			return false, "oidc_discovery", "ISSUER_MISMATCH", `{"reason":"discovery issuer mismatch"}`
		}
		return true, "oidc_discovery", "", `{"issuer_verified":true}`
	}
	return true, "binding_structure", "", `{"binding_verified":true}`
}

func requestID(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return ""
}

type requestIDContextKey struct{}

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
	if err := portaldomain.ValidateApplication(portaldomain.Application{Code: input.Code, Name: input.Name, LaunchType: input.LaunchType, HomeURL: input.HomeURL, LaunchURL: input.LaunchURL, Status: input.Status}); err != nil {
		return portaldomain.Application{}, err
	}
	item, err := s.repo.CreateApplication(ctx, principal.OrganizationID, principal.UserID, input)
	if err != nil {
		return portaldomain.Application{}, err
	}
	if !strings.EqualFold(input.LaunchType, "URL") {
		item, err = s.repo.SetApplicationLifecycle(ctx, principal.OrganizationID, principal.UserID, item.ID, portaldomain.LifecycleIdentityPending, portaldomain.StatusDisabled, item.ConfigVersion, false)
	}
	return item, err
}

func (s *Service) UpdateApplication(ctx context.Context, principal domain.Principal, id string, input repository.ApplicationInput) (portaldomain.Application, error) {
	existing, err := s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Application{}, ErrNotFound
	}
	if err != nil {
		return portaldomain.Application{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = portaldomain.StatusEnabled
	}
	if input.LaunchType == "" {
		input.LaunchType = "URL"
	}
	if err := portaldomain.ValidateApplication(portaldomain.Application{Code: "valid", Name: input.Name, LaunchType: input.LaunchType, HomeURL: input.HomeURL, LaunchURL: input.LaunchURL, Status: input.Status}); err != nil {
		return portaldomain.Application{}, err
	}
	item, err := s.repo.UpdateApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(id), input)
	if errors.Is(err, sql.ErrNoRows) {
		return portaldomain.Application{}, ErrNotFound
	}
	if err == nil {
		verified := false
		if !strings.EqualFold(input.LaunchType, "URL") && strings.EqualFold(existing.LaunchType, input.LaunchType) {
			binding, bindingErr := s.repo.GetIdentityBinding(ctx, principal.OrganizationID, item.ID)
			verified = bindingErr == nil && binding.VerificationStatus == portaldomain.VerificationPassed
		}
		lifecycle, status, publish := applicationLifecycleAfterUpdate(existing, input, verified)
		item, err = s.repo.SetApplicationLifecycle(ctx, principal.OrganizationID, principal.UserID, item.ID, lifecycle, status, item.ConfigVersion, publish)
	}
	return item, err
}

// applicationLifecycleAfterUpdate keeps ordinary metadata edits from
// invalidating an already published OIDC application. Identity onboarding is
// restarted only when the launch protocol changes; a previously verified
// binding may also recover an application disabled by the legacy edit bug.
func applicationLifecycleAfterUpdate(existing portaldomain.Application, input repository.ApplicationInput, bindingVerified bool) (string, string, bool) {
	requestedStatus := strings.ToUpper(strings.TrimSpace(input.Status))
	if requestedStatus != portaldomain.StatusDisabled {
		requestedStatus = portaldomain.StatusEnabled
	}
	if strings.EqualFold(input.LaunchType, "URL") {
		return portaldomain.LifecyclePublished, requestedStatus, requestedStatus == portaldomain.StatusEnabled
	}
	if !strings.EqualFold(existing.LaunchType, input.LaunchType) {
		return portaldomain.LifecycleIdentityPending, portaldomain.StatusDisabled, false
	}
	if existing.LifecycleStatus == portaldomain.LifecyclePublished || bindingVerified {
		return portaldomain.LifecyclePublished, requestedStatus, requestedStatus == portaldomain.StatusEnabled
	}
	lifecycle := existing.LifecycleStatus
	if lifecycle == "" || lifecycle == portaldomain.LifecycleDisabled {
		lifecycle = portaldomain.LifecycleIdentityPending
	}
	return lifecycle, portaldomain.StatusDisabled, false
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

func (s *Service) ListApplicationRoles(ctx context.Context, principal domain.Principal, appID string) ([]portaldomain.ApplicationRole, error) {
	if _, err := s.repo.GetApplication(ctx, principal.OrganizationID, principal.UserID, strings.TrimSpace(appID), true); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return s.repo.ListApplicationRoles(ctx, principal.OrganizationID, strings.TrimSpace(appID))
}

func (s *Service) ReplaceApplicationRoles(ctx context.Context, principal domain.Principal, appID string, roles []portaldomain.ApplicationRole) ([]portaldomain.ApplicationRole, error) {
	seen := make(map[string]struct{}, len(roles))
	for i := range roles {
		roles[i].Key = strings.ToLower(strings.TrimSpace(roles[i].Key))
		roles[i].Name = strings.TrimSpace(roles[i].Name)
		roles[i].Description = strings.TrimSpace(roles[i].Description)
		roles[i].RiskLevel = strings.ToUpper(strings.TrimSpace(roles[i].RiskLevel))
		roles[i].Status = strings.ToUpper(strings.TrimSpace(roles[i].Status))
		if roles[i].Status == "" {
			roles[i].Status = portaldomain.StatusActive
		}
		if roles[i].RiskLevel == "" {
			roles[i].RiskLevel = portaldomain.RoleRiskNormal
		}
		if roles[i].Key == "" || len(roles[i].Key) > 100 || roles[i].Name == "" || len(roles[i].Name) > 200 || len(roles[i].Description) > 500 || !validApplicationRoleKey(roles[i].Key) {
			return nil, ErrInvalid
		}
		if roles[i].RiskLevel != portaldomain.RoleRiskNormal && roles[i].RiskLevel != portaldomain.RoleRiskPrivileged && roles[i].RiskLevel != portaldomain.RoleRiskCritical {
			return nil, ErrInvalid
		}
		if roles[i].Status != portaldomain.StatusActive && roles[i].Status != portaldomain.StatusDisabled {
			return nil, ErrInvalid
		}
		if _, exists := seen[roles[i].Key]; exists {
			return nil, ErrInvalid
		}
		seen[roles[i].Key] = struct{}{}
	}
	items, err := s.repo.ReplaceApplicationRoles(ctx, principal.OrganizationID, strings.TrimSpace(appID), roles)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return items, err
}

func validApplicationRoleKey(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r != '_' && r != '-' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
