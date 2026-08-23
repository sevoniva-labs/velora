package kratosapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	approvalapp "github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	appportal "github.com/sevoniva-labs/velora/server/internal/app/portal"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
	"github.com/sevoniva-labs/velora/server/internal/platform/casdooradmin"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpserver"
	"github.com/sevoniva-labs/velora/server/internal/platform/idempotency"
	"google.golang.org/protobuf/proto"
)

type PortalService struct {
	forgev1.UnimplementedPortalServiceServer
	portal                    *appportal.Service
	audit                     *audit.Writer
	db                        *database.DB
	identityAdminURL          string
	identityIssuer            string
	identityInternalURL       string
	identityAllowedHosts      []string
	identityOnboardingEnabled bool
	identityAdminEntryEnabled bool
	casdoorAutomation         casdooradmin.ApplicationProvider
	approval                  *approvalapp.Service
	idem                      *idempotency.Store
}

func NewPortalService(portal *appportal.Service, auditWriter *audit.Writer, db *database.DB) *PortalService {
	return &PortalService{portal: portal, audit: auditWriter, db: db}
}

func (s *PortalService) ConfigureIdentityBoundary(adminURL, issuer, internalURL string, allowedHosts []string, onboardingEnabled, adminEntryEnabled bool) {
	s.identityAdminURL = strings.TrimSpace(adminURL)
	s.identityIssuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	s.identityInternalURL = strings.TrimRight(strings.TrimSpace(internalURL), "/")
	s.identityAllowedHosts = append([]string(nil), allowedHosts...)
	s.identityOnboardingEnabled = onboardingEnabled
	s.identityAdminEntryEnabled = adminEntryEnabled
}

func (s *PortalService) ConfigureCasdoorAutomation(provider casdooradmin.ApplicationProvider) {
	s.casdoorAutomation = provider
}

func (s *PortalService) ConfigureApproval(service *approvalapp.Service) {
	s.approval = service
}

func (s *PortalService) ConfigureIdempotency(store *idempotency.Store) {
	s.idem = store
}

func (s *PortalService) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
	if s.db == nil || s.audit == nil {
		return errors.New("reliable audit is unavailable")
	}
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := operation(txCtx); err != nil {
			return err
		}
		return s.audit.Write(txCtx, *event)
	})
}

// AuthorizePortalApplication is the trusted ForwardAuth boundary for legacy
// applications. The application ID is taken only from the authenticated
// request route and is checked through the same CanAccess path as the portal.
// Gateways must strip inbound X-Velora-* headers and copy only response
// headers emitted by this endpoint to the upstream application.
func (s *PortalService) AuthorizePortalApplication(ctx context.Context, req *forgev1.AuthorizePortalApplicationRequest) (*forgev1.AuthorizePortalApplicationResponse, error) {
	if !httpserver.IsTrustedProxy(ctx) {
		return nil, kratoserrors.Forbidden("FORWARD_AUTH_PROXY_UNTRUSTED", "forward auth requires a trusted gateway")
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.portal.GetApplication(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	targetURL := item.HomeURL
	if strings.TrimSpace(targetURL) == "" {
		targetURL = item.LaunchURL
	}
	if !forwardAuthHostMatches(targetURL, requestHeader(ctx, "X-Forwarded-Host", 255)) {
		return nil, kratoserrors.Forbidden("FORWARD_AUTH_HOST_INVALID", "forward auth host is not bound to the registered application")
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		h := tr.ReplyHeader()
		h.Set("X-Velora-Authenticated", "true")
		h.Set("X-Velora-Application-ID", item.ID)
		h.Set("X-Velora-User-ID", principal.UserID)
		h.Set("X-Velora-Login-Name", principal.LoginName)
		h.Set("X-Velora-Organization-ID", principal.OrganizationID)
	}
	return &forgev1.AuthorizePortalApplicationResponse{
		ApplicationId:  item.ID,
		UserId:         principal.UserID,
		LoginName:      principal.LoginName,
		OrganizationId: principal.OrganizationID,
		DisplayName:    principal.DisplayName,
	}, nil
}

func forwardAuthHostMatches(targetURL, forwardedHost string) bool {
	forwardedHost = strings.TrimSpace(forwardedHost)
	if forwardedHost == "" || strings.ContainsAny(forwardedHost, ", \t\r\n/\\") {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil {
		return false
	}
	return strings.EqualFold(u.Host, forwardedHost)
}

func (s *PortalService) ListPortalApplications(ctx context.Context, req *forgev1.ListPortalApplicationsRequest) (*forgev1.ListPortalApplicationsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.ListApplications(ctx, principal, repository.ApplicationFilter{Keyword: req.GetKeyword(), CategoryID: req.GetCategoryId(), TagID: req.GetTagId(), FavoritesOnly: req.GetFavoritesOnly(), Limit: int(req.GetLimit())})
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.ListPortalApplicationsResponse{Applications: portalApplicationsProto(items)}, nil
}

func (s *PortalService) GetPortalApplication(ctx context.Context, req *forgev1.GetPortalApplicationRequest) (*forgev1.GetPortalApplicationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.portal.GetApplication(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.GetPortalApplicationResponse{Application: portalApplicationProto(item)}, nil
}

func (s *PortalService) LaunchPortalApplication(ctx context.Context, req *forgev1.LaunchPortalApplicationRequest) (*forgev1.LaunchPortalApplicationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var item portaldomain.Application
	var launchURL string
	event := newAuditEvent(ctx, principal, "portal.application.launch", "portal_application", req.GetApplicationId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var launchErr error
		item, launchURL, launchErr = s.portal.Launch(txCtx, principal, req.GetApplicationId())
		if launchErr == nil {
			event.ResourceID = item.ID
		}
		return launchErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.LaunchPortalApplicationResponse{Application: portalApplicationProto(item), LaunchUrl: launchURL}, nil
}

func (s *PortalService) ListPortalFavorites(ctx context.Context, req *forgev1.ListPortalFavoritesRequest) (*forgev1.ListPortalFavoritesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.ListFavorites(ctx, principal, int(req.GetLimit()))
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.ListPortalFavoritesResponse{Applications: portalApplicationsProto(items)}, nil
}

func (s *PortalService) AddPortalFavorite(ctx context.Context, req *forgev1.AddPortalFavoriteRequest) (*forgev1.AddPortalFavoriteResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var item portaldomain.Application
	event := newAuditEvent(ctx, principal, "portal.favorite.add", "portal_application", req.GetApplicationId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var addErr error
		item, addErr = s.portal.AddFavorite(txCtx, principal, req.GetApplicationId())
		return addErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.AddPortalFavoriteResponse{Application: portalApplicationProto(item)}, nil
}

func (s *PortalService) RemovePortalFavorite(ctx context.Context, req *forgev1.RemovePortalFavoriteRequest) (*forgev1.RemovePortalFavoriteResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "portal.favorite.remove", "portal_application", req.GetApplicationId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.portal.RemoveFavorite(txCtx, principal, req.GetApplicationId())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RemovePortalFavoriteResponse{}, nil
}

func (s *PortalService) ListRecentPortalApplications(ctx context.Context, req *forgev1.ListRecentPortalApplicationsRequest) (*forgev1.ListRecentPortalApplicationsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.ListRecent(ctx, principal, int(req.GetLimit()))
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.ListRecentPortalApplicationsResponse{Applications: portalApplicationsProto(items)}, nil
}

func (s *PortalService) ListPortalCategories(ctx context.Context, _ *forgev1.ListPortalCategoriesRequest) (*forgev1.ListPortalCategoriesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.Categories(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.ListPortalCategoriesResponse{Categories: portalCategoriesProto(items)}, nil
}

func (s *PortalService) ListPortalTags(ctx context.Context, _ *forgev1.ListPortalTagsRequest) (*forgev1.ListPortalTagsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.Tags(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.ListPortalTagsResponse{Tags: portalTagsProto(items)}, nil
}

func (s *PortalService) ListAdminPortalApplications(ctx context.Context, req *forgev1.ListAdminPortalApplicationsRequest) (*forgev1.ListAdminPortalApplicationsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.AdminListApplications(ctx, principal, int(req.GetLimit()))
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.ListAdminPortalApplicationsResponse{Applications: portalApplicationsProto(items)}, nil
}

func (s *PortalService) CreatePortalApplication(ctx context.Context, req *forgev1.CreatePortalApplicationRequest) (*forgev1.CreatePortalApplicationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.application.create", req, func() proto.Message { return &forgev1.CreatePortalApplicationResponse{} }, func() (proto.Message, error) {
		var item portaldomain.Application
		event := newAuditEvent(ctx, principal, "portal.application.create", "portal_application", "", map[string]any{"code": req.GetCode()})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var createErr error
			item, createErr = s.portal.CreateApplication(txCtx, principal, applicationInput(req))
			if createErr == nil {
				event.ResourceID = item.ID
			}
			return createErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.CreatePortalApplicationResponse{Application: portalApplicationProto(item)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.CreatePortalApplicationResponse), nil
}

func (s *PortalService) UpdatePortalApplication(ctx context.Context, req *forgev1.UpdatePortalApplicationRequest) (*forgev1.UpdatePortalApplicationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.application.update", req, func() proto.Message { return &forgev1.UpdatePortalApplicationResponse{} }, func() (proto.Message, error) {
		var item portaldomain.Application
		event := newAuditEvent(ctx, principal, "portal.application.update", "portal_application", req.GetApplicationId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var updateErr error
			item, updateErr = s.portal.UpdateApplication(txCtx, principal, req.GetApplicationId(), repository.ApplicationInput{Name: req.GetName(), Description: req.GetDescription(), Icon: req.GetIcon(), CategoryID: req.GetCategoryId(), HomeURL: req.GetHomeUrl(), LaunchURL: req.GetLaunchUrl(), LaunchType: req.GetLaunchType(), Status: req.GetStatus(), SortOrder: int(req.GetSortOrder()), Featured: req.GetFeatured(), TagIDs: req.GetTagIds()})
			return updateErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.UpdatePortalApplicationResponse{Application: portalApplicationProto(item)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.UpdatePortalApplicationResponse), nil
}

func (s *PortalService) DeletePortalApplication(ctx context.Context, req *forgev1.DeletePortalApplicationRequest) (*forgev1.DeletePortalApplicationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.application.delete", req, func() proto.Message { return &forgev1.DeletePortalApplicationResponse{} }, func() (proto.Message, error) {
		event := newAuditEvent(ctx, principal, "portal.application.delete", "portal_application", req.GetApplicationId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			return s.portal.DeleteApplication(txCtx, principal, req.GetApplicationId())
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.DeletePortalApplicationResponse{}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.DeletePortalApplicationResponse), nil
}

func (s *PortalService) CreatePortalCategory(ctx context.Context, req *forgev1.CreatePortalCategoryRequest) (*forgev1.CreatePortalCategoryResponse, error) {
	return s.createCategory(ctx, req)
}

func (s *PortalService) UpdatePortalCategory(ctx context.Context, req *forgev1.UpdatePortalCategoryRequest) (*forgev1.UpdatePortalCategoryResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.category.update", req, func() proto.Message { return &forgev1.UpdatePortalCategoryResponse{} }, func() (proto.Message, error) {
		var item portaldomain.Category
		event := newAuditEvent(ctx, principal, "portal.category.update", "portal_category", req.GetCategoryId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var updateErr error
			item, updateErr = s.portal.UpdateCategory(txCtx, principal, req.GetCategoryId(), repository.CategoryInput{Name: req.GetName(), Description: req.GetDescription(), SortOrder: int(req.GetSortOrder()), Status: req.GetStatus()})
			return updateErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.UpdatePortalCategoryResponse{Category: portalCategoryProto(item)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.UpdatePortalCategoryResponse), nil
}

func (s *PortalService) DeletePortalCategory(ctx context.Context, req *forgev1.DeletePortalCategoryRequest) (*forgev1.DeletePortalCategoryResponse, error) {
	return s.deleteCategory(ctx, req)
}

func (s *PortalService) CreatePortalTag(ctx context.Context, req *forgev1.CreatePortalTagRequest) (*forgev1.CreatePortalTagResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.tag.create", req, func() proto.Message { return &forgev1.CreatePortalTagResponse{} }, func() (proto.Message, error) {
		var item portaldomain.Tag
		event := newAuditEvent(ctx, principal, "portal.tag.create", "portal_tag", "", map[string]any{"tag_key": req.GetTagKey()})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var createErr error
			item, createErr = s.portal.CreateTag(txCtx, principal, repository.TagInput{Key: req.GetTagKey(), Name: req.GetName(), SortOrder: int(req.GetSortOrder())})
			if createErr == nil {
				event.ResourceID = item.ID
			}
			return createErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.CreatePortalTagResponse{Tag: portalTagProto(item)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.CreatePortalTagResponse), nil
}

func (s *PortalService) UpdatePortalTag(ctx context.Context, req *forgev1.UpdatePortalTagRequest) (*forgev1.UpdatePortalTagResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.tag.update", req, func() proto.Message { return &forgev1.UpdatePortalTagResponse{} }, func() (proto.Message, error) {
		var item portaldomain.Tag
		event := newAuditEvent(ctx, principal, "portal.tag.update", "portal_tag", req.GetTagId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var updateErr error
			item, updateErr = s.portal.UpdateTag(txCtx, principal, req.GetTagId(), repository.TagInput{Name: req.GetName(), SortOrder: int(req.GetSortOrder())})
			return updateErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.UpdatePortalTagResponse{Tag: portalTagProto(item)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.UpdatePortalTagResponse), nil
}

func (s *PortalService) DeletePortalTag(ctx context.Context, req *forgev1.DeletePortalTagRequest) (*forgev1.DeletePortalTagResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.tag.delete", req, func() proto.Message { return &forgev1.DeletePortalTagResponse{} }, func() (proto.Message, error) {
		event := newAuditEvent(ctx, principal, "portal.tag.delete", "portal_tag", req.GetTagId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error { return s.portal.DeleteTag(txCtx, principal, req.GetTagId()) }); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.DeletePortalTagResponse{}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.DeletePortalTagResponse), nil
}

func (s *PortalService) ReplacePortalApplicationPolicies(ctx context.Context, req *forgev1.ReplacePortalApplicationPoliciesRequest) (*forgev1.ReplacePortalApplicationPoliciesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.policy.replace", req, func() proto.Message { return &forgev1.ReplacePortalApplicationPoliciesResponse{} }, func() (proto.Message, error) {
		policies := make([]portaldomain.AccessPolicy, 0, len(req.GetPolicies()))
		for _, item := range req.GetPolicies() {
			if item != nil {
				policies = append(policies, portaldomain.AccessPolicy{Type: item.GetPolicyType(), Value: item.GetValue()})
			}
		}
		var out []portaldomain.AccessPolicy
		event := newAuditEvent(ctx, principal, "portal.policy.replace", "portal_application", req.GetApplicationId(), map[string]any{"policy_count": len(policies)})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var replaceErr error
			out, replaceErr = s.portal.ReplacePolicies(txCtx, principal, req.GetApplicationId(), policies)
			return replaceErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.ReplacePortalApplicationPoliciesResponse{Policies: portalPoliciesProto(out)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.ReplacePortalApplicationPoliciesResponse), nil
}

func (s *PortalService) ListPortalApplicationRoles(ctx context.Context, req *forgev1.ListPortalApplicationRolesRequest) (*forgev1.ListPortalApplicationRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.portal.ListApplicationRoles(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ListPortalApplicationRolesResponse{Roles: portalApplicationRolesProto(roles)}, nil
}

func (s *PortalService) ReplacePortalApplicationRoles(ctx context.Context, req *forgev1.ReplacePortalApplicationRolesRequest) (*forgev1.ReplacePortalApplicationRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.application.roles.replace", req, func() proto.Message { return &forgev1.ReplacePortalApplicationRolesResponse{} }, func() (proto.Message, error) {
		roles := make([]portaldomain.ApplicationRole, 0, len(req.GetRoles()))
		for _, item := range req.GetRoles() {
			if item != nil {
				roles = append(roles, portaldomain.ApplicationRole{Key: item.GetRoleKey(), Name: item.GetName(), Description: item.GetDescription(), RiskLevel: item.GetRiskLevel(), Status: item.GetStatus()})
			}
		}
		var out []portaldomain.ApplicationRole
		event := newAuditEvent(ctx, principal, "portal.application.roles.replace", "portal_application", req.GetApplicationId(), map[string]any{"role_count": len(roles)})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var replaceErr error
			out, replaceErr = s.portal.ReplaceApplicationRoles(txCtx, principal, req.GetApplicationId(), roles)
			return replaceErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.ReplacePortalApplicationRolesResponse{Roles: portalApplicationRolesProto(out)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.ReplacePortalApplicationRolesResponse), nil
}

func (s *PortalService) GetIdentityOverview(ctx context.Context, _ *forgev1.GetIdentityOverviewRequest) (*forgev1.GetIdentityOverviewResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	host := ""
	if parsed, err := url.Parse(s.identityAdminURL); err == nil {
		host = parsed.Hostname()
	}
	pending := int64(0)
	items, listErr := s.portal.AdminListApplications(ctx, principal, 1000)
	if listErr != nil {
		return nil, internalError(listErr)
	}
	for _, item := range items {
		switch item.LifecycleStatus {
		case portaldomain.LifecycleDraft, portaldomain.LifecycleIdentityPending, portaldomain.LifecycleVerificationPending, portaldomain.LifecycleVerificationFailed, portaldomain.LifecycleReady:
			pending++
		}
	}
	connectionStatus := identityConnectionStatus(ctx, s.identityIssuer, s.identityInternalURL)
	return &forgev1.GetIdentityOverviewResponse{OnboardingEnabled: s.identityOnboardingEnabled, AdminEntryEnabled: s.identityAdminEntryEnabled, ProviderKey: portaldomain.IdentityProviderCasdoor, AdminUrlHost: host, Issuer: s.identityIssuer, ConnectionStatus: connectionStatus, PendingApplicationCount: pending, AutomationEnabled: s.casdoorAutomation != nil && s.casdoorAutomation.Enabled()}, nil
}

func identityConnectionStatus(ctx context.Context, issuer, internalURL string) string {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return "UNCONFIGURED"
	}
	target := strings.TrimRight(strings.TrimSpace(internalURL), "/")
	if target == "" {
		target = issuer
	}
	u, err := url.Parse(target + "/.well-known/openid-configuration")
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "UNAVAILABLE"
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "UNAVAILABLE"
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "UNAVAILABLE"
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "UNAVAILABLE"
	}
	var metadata struct {
		Issuer string `json:"issuer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil || strings.TrimRight(strings.TrimSpace(metadata.Issuer), "/") != issuer {
		return "MISMATCH"
	}
	return "CONNECTED"
}

func (s *PortalService) GetIdentityConsoleLink(ctx context.Context, _ *forgev1.GetIdentityConsoleLinkRequest) (*forgev1.GetIdentityConsoleLinkResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !s.identityAdminEntryEnabled || !safeIdentityAdminURL(s.identityAdminURL, s.identityAllowedHosts) {
		return nil, kratoserrors.NotFound("IDENTITY_CONSOLE_UNAVAILABLE", "identity console is not available")
	}
	event := newAuditEvent(ctx, principal, "iam.console.open", "identity_console", "", map[string]any{"provider": portaldomain.IdentityProviderCasdoor})
	if err := s.audited(ctx, event, func(context.Context) error { return nil }); err != nil {
		return nil, internalError(err)
	}
	return &forgev1.GetIdentityConsoleLinkResponse{Url: s.identityAdminURL, ProviderKey: portaldomain.IdentityProviderCasdoor}, nil
}

func (s *PortalService) GetApplicationOnboarding(ctx context.Context, req *forgev1.GetApplicationOnboardingRequest) (*forgev1.GetApplicationOnboardingResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	app, binding, verifications, err := s.portal.GetApplicationOnboarding(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	canPublish := strings.EqualFold(app.LaunchType, "URL") || (binding.ID != "" && binding.VerificationStatus == portaldomain.VerificationPassed && app.LifecycleStatus == portaldomain.LifecycleReady)
	return &forgev1.GetApplicationOnboardingResponse{Application: portalApplicationProto(app), Binding: identityBindingProto(binding), Verifications: verificationsProto(verifications), CanPublish: canPublish}, nil
}

func (s *PortalService) UpsertApplicationIdentityBinding(ctx context.Context, req *forgev1.UpsertApplicationIdentityBindingRequest) (*forgev1.UpsertApplicationIdentityBindingResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !principal.HasPermission("iam.integration.manage") {
		return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "identity integration management permission is required")
	}
	automationEnabled := s.casdoorAutomation != nil && s.casdoorAutomation.Enabled() && strings.EqualFold(req.GetProviderKey(), portaldomain.IdentityProviderCasdoor) && strings.EqualFold(req.GetProtocol(), portaldomain.ProtocolOIDC)
	if automationEnabled && strings.TrimSpace(req.GetApprovalId()) == "" {
		return nil, serviceError(casdooradmin.ErrApprovalRequired)
	}
	response, err := s.idempotentWith(ctx, principal, "iam.integration.update", req, func() proto.Message { return &forgev1.UpsertApplicationIdentityBindingResponse{} }, func() (proto.Message, error) {
		var automationScopes []string
		if automationEnabled {
			var scopeErr error
			automationScopes, scopeErr = portaldomain.NormalizeOIDCScopes(req.GetScopes())
			if scopeErr != nil {
				return nil, serviceError(scopeErr)
			}
			if err := s.authorizeCasdoorAutomation(ctx, principal, req.GetApprovalId(), "UPSERT", req.GetApplicationId(), map[string]any{
				"provider":                 portaldomain.IdentityProviderCasdoor,
				"protocol":                 req.GetProtocol(),
				"provider_application_ref": req.GetProviderApplicationRef(),
				"public_client_id":         req.GetPublicClientId(),
				"issuer":                   req.GetIssuer(),
				"redirect_uris":            req.GetRedirectUris(),
				"scopes":                   automationScopes,
			}); err != nil {
				return nil, serviceError(err)
			}
		}
		var binding portaldomain.IdentityBinding
		var app portaldomain.Application
		var oneTimeClientSecret string
		event := newAuditEvent(ctx, principal, "iam.integration.update", "portal_application", req.GetApplicationId(), map[string]any{"protocol": req.GetProtocol(), "provider": portaldomain.IdentityProviderCasdoor})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			binding, app, operationErr = s.portal.UpsertApplicationIdentityBinding(txCtx, principal, req.GetApplicationId(), portaldomain.IdentityBindingInput{ProviderKey: req.GetProviderKey(), Protocol: req.GetProtocol(), ProviderApplicationRef: req.GetProviderApplicationRef(), PublicClientID: req.GetPublicClientId(), Issuer: req.GetIssuer(), RedirectURIs: req.GetRedirectUris(), Scopes: req.GetScopes()}, req.GetExpectedConfigVersion())
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		// Keep the Velora binding in VERIFICATION_PENDING when the external call
		// fails. A later retry is safe because the Casdoor provider upsert is
		// idempotent and the local binding is already auditable and recoverable.
		if automationEnabled {
			application, _, automationErr := s.casdoorAutomation.UpsertApplication(ctx, casdooradmin.UpsertInput{Name: req.GetProviderApplicationRef(), Organization: principal.OrganizationID, DisplayName: req.GetProviderApplicationRef(), ClientID: req.GetPublicClientId(), RedirectURIs: req.GetRedirectUris(), GrantTypes: []string{"authorization_code"}, Scopes: automationScopes, ApprovalID: req.GetApprovalId()})
			if automationErr != nil {
				return nil, serviceError(automationErr)
			}
			oneTimeClientSecret = application.TakeOneTimeClientSecret()
		}
		return &forgev1.UpsertApplicationIdentityBindingResponse{Binding: identityBindingProto(binding), Application: portalApplicationProto(app), OneTimeClientSecret: oneTimeClientSecret}, nil
	}, func(message proto.Message) proto.Message {
		cached := proto.Clone(message).(*forgev1.UpsertApplicationIdentityBindingResponse)
		cached.OneTimeClientSecret = ""
		return cached
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.UpsertApplicationIdentityBindingResponse), nil
}

func (s *PortalService) VerifyApplicationIdentity(ctx context.Context, req *forgev1.VerifyApplicationIdentityRequest) (*forgev1.VerifyApplicationIdentityResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "iam.integration.verify", req, func() proto.Message { return &forgev1.VerifyApplicationIdentityResponse{} }, func() (proto.Message, error) {
		var binding portaldomain.IdentityBinding
		var app portaldomain.Application
		var verifications []portaldomain.Verification
		var passed bool
		event := newAuditEvent(ctx, principal, "iam.integration.verify", "portal_application", req.GetApplicationId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			binding, app, verifications, passed, operationErr = s.portal.VerifyApplicationIdentity(txCtx, principal, req.GetApplicationId(), req.GetExpectedConfigVersion())
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.VerifyApplicationIdentityResponse{Binding: identityBindingProto(binding), Application: portalApplicationProto(app), Verifications: verificationsProto(verifications), Passed: passed}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.VerifyApplicationIdentityResponse), nil
}

func (s *PortalService) SubmitApplicationPublish(ctx context.Context, req *forgev1.SubmitApplicationPublishRequest) (*forgev1.SubmitApplicationPublishResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.application.submit_publish", req, func() proto.Message { return &forgev1.SubmitApplicationPublishResponse{} }, func() (proto.Message, error) {
		var app portaldomain.Application
		event := newAuditEvent(ctx, principal, "portal.application.submit_publish", "portal_application", req.GetApplicationId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			app, operationErr = s.portal.SubmitApplicationPublish(txCtx, principal, req.GetApplicationId(), req.GetExpectedConfigVersion())
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.SubmitApplicationPublishResponse{Application: portalApplicationProto(app)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.SubmitApplicationPublishResponse), nil
}

func (s *PortalService) PublishApplication(ctx context.Context, req *forgev1.PublishApplicationRequest) (*forgev1.PublishApplicationResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.application.publish", req, func() proto.Message { return &forgev1.PublishApplicationResponse{} }, func() (proto.Message, error) {
		var app portaldomain.Application
		event := newAuditEvent(ctx, principal, "portal.application.publish", "portal_application", req.GetApplicationId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			app, operationErr = s.portal.PublishApplication(txCtx, principal, req.GetApplicationId(), req.GetExpectedConfigVersion())
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.PublishApplicationResponse{Application: portalApplicationProto(app)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.PublishApplicationResponse), nil
}

func (s *PortalService) DisableApplication(ctx context.Context, req *forgev1.DisableApplicationRequest) (*forgev1.DisableApplicationResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	_, binding, _, onboardingErr := s.portal.GetApplicationOnboarding(ctx, principal, req.GetApplicationId())
	if onboardingErr != nil {
		return nil, serviceError(onboardingErr)
	}
	automationEnabled := s.casdoorAutomation != nil && s.casdoorAutomation.Enabled() && binding.ID != "" && strings.EqualFold(binding.ProviderKey, portaldomain.IdentityProviderCasdoor)
	if automationEnabled {
		if !principal.HasPermission("iam.console.open") {
			return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "identity administrator permission is required")
		}
		if strings.TrimSpace(req.GetApprovalId()) == "" {
			return nil, serviceError(casdooradmin.ErrApprovalRequired)
		}
		if err := s.authorizeCasdoorAutomation(ctx, principal, req.GetApprovalId(), "DISABLE", req.GetApplicationId(), map[string]any{
			"provider":                 portaldomain.IdentityProviderCasdoor,
			"provider_application_ref": binding.ProviderApplicationRef,
		}); err != nil {
			return nil, serviceError(err)
		}
	}
	response, err := s.idempotent(ctx, principal, "portal.application.disable", req, func() proto.Message { return &forgev1.DisableApplicationResponse{} }, func() (proto.Message, error) {
		var app portaldomain.Application
		event := newAuditEvent(ctx, principal, "portal.application.disable", "portal_application", req.GetApplicationId(), map[string]any{"approval_id": req.GetApprovalId(), "casdoor_automation": automationEnabled})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			app, operationErr = s.portal.DisableApplication(txCtx, principal, req.GetApplicationId(), req.GetExpectedConfigVersion())
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		if automationEnabled {
			if err := s.casdoorAutomation.DisableApplication(ctx, binding.ProviderApplicationRef, req.GetApprovalId()); err != nil {
				return nil, serviceError(err)
			}
		}
		return &forgev1.DisableApplicationResponse{Application: portalApplicationProto(app)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.DisableApplicationResponse), nil
}

func (s *PortalService) identityOnboardingRequired() error {
	if !s.identityOnboardingEnabled {
		return kratoserrors.ServiceUnavailable("APPLICATION_ONBOARDING_DISABLED", "application onboarding is disabled by configuration")
	}
	return nil
}

func (s *PortalService) authorizeCasdoorAutomation(ctx context.Context, principal domain.Principal, approvalID, action, resourceID string, payload map[string]any) error {
	if s.approval == nil {
		return errors.New("approval service is unavailable")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return errors.New("automation approval payload is invalid")
	}
	return s.approval.AuthorizeExecution(ctx, principal, approvalID, approvalapp.ExecutionInput{
		RequestType: "CASDOOR_APPLICATION",
		Action:      action,
		Resource:    "portal_application",
		ResourceID:  resourceID,
		PayloadJSON: string(encoded),
	})
}

func (s *PortalService) createCategory(ctx context.Context, req *forgev1.CreatePortalCategoryRequest) (*forgev1.CreatePortalCategoryResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.category.create", req, func() proto.Message { return &forgev1.CreatePortalCategoryResponse{} }, func() (proto.Message, error) {
		var item portaldomain.Category
		input := repository.CategoryInput{Key: req.GetCategoryKey(), Name: req.GetName(), Description: req.GetDescription(), SortOrder: int(req.GetSortOrder()), Status: req.GetStatus()}
		event := newAuditEvent(ctx, principal, "portal.category.create", "portal_category", "", map[string]any{"category_key": input.Key})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var createErr error
			item, createErr = s.portal.CreateCategory(txCtx, principal, input)
			if createErr == nil {
				event.ResourceID = item.ID
			}
			return createErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.CreatePortalCategoryResponse{Category: portalCategoryProto(item)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.CreatePortalCategoryResponse), nil
}

func (s *PortalService) deleteCategory(ctx context.Context, req *forgev1.DeletePortalCategoryRequest) (*forgev1.DeletePortalCategoryResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotent(ctx, principal, "portal.category.delete", req, func() proto.Message { return &forgev1.DeletePortalCategoryResponse{} }, func() (proto.Message, error) {
		event := newAuditEvent(ctx, principal, "portal.category.delete", "portal_category", req.GetCategoryId(), nil)
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			return s.portal.DeleteCategory(txCtx, principal, req.GetCategoryId())
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.DeletePortalCategoryResponse{}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.DeletePortalCategoryResponse), nil
}

func applicationInput(req *forgev1.CreatePortalApplicationRequest) repository.ApplicationInput {
	return repository.ApplicationInput{Code: req.GetCode(), Name: req.GetName(), Description: req.GetDescription(), Icon: req.GetIcon(), CategoryID: req.GetCategoryId(), HomeURL: req.GetHomeUrl(), LaunchURL: req.GetLaunchUrl(), LaunchType: req.GetLaunchType(), Status: req.GetStatus(), SortOrder: int(req.GetSortOrder()), Featured: req.GetFeatured(), TagIDs: req.GetTagIds()}
}

func portalApplicationsProto(items []portaldomain.Application) []*forgev1.PortalApplication {
	out := make([]*forgev1.PortalApplication, 0, len(items))
	for _, item := range items {
		out = append(out, portalApplicationProto(item))
	}
	return out
}

func portalApplicationProto(item portaldomain.Application) *forgev1.PortalApplication {
	return &forgev1.PortalApplication{Id: item.ID, OrganizationId: item.OrganizationID, Code: item.Code, Name: item.Name, Description: item.Description, Icon: item.Icon, CategoryId: item.CategoryID, CategoryName: item.CategoryName, HomeUrl: item.HomeURL, LaunchUrl: item.LaunchURL, LaunchType: item.LaunchType, Status: item.Status, SortOrder: int64(item.SortOrder), Featured: item.Featured, Favorite: item.Favorite, VisitCount: item.VisitCount, Tags: portalTagsProto(item.Tags), Policies: portalPoliciesProto(item.Policies), CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt), LifecycleStatus: item.LifecycleStatus, ConfigVersion: item.ConfigVersion, PublishedAt: optionalTimestamp(item.PublishedAt), PublishedBy: item.PublishedBy}
}

func identityBindingProto(item portaldomain.IdentityBinding) *forgev1.PortalIdentityBinding {
	if item.ID == "" {
		return nil
	}
	return &forgev1.PortalIdentityBinding{Id: item.ID, OrganizationId: item.OrganizationID, ApplicationId: item.ApplicationID, ProviderKey: item.ProviderKey, Protocol: item.Protocol, ProviderApplicationRef: item.ProviderApplicationRef, PublicClientId: item.PublicClientID, Issuer: item.Issuer, RedirectUris: item.RedirectURIs, Scopes: item.Scopes, ConfigurationStatus: item.ConfigurationStatus, VerificationStatus: item.VerificationStatus, VerifiedAt: optionalTimestamp(item.VerifiedAt), VerifiedBy: item.VerifiedBy, VerificationError: item.VerificationError, ConfigVersion: item.ConfigVersion, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)}
}

func verificationsProto(items []portaldomain.Verification) []*forgev1.PortalApplicationVerification {
	out := make([]*forgev1.PortalApplicationVerification, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalApplicationVerification{Id: item.ID, ApplicationId: item.ApplicationID, BindingId: item.BindingID, CheckType: item.CheckType, Result: item.Result, ErrorCode: item.ErrorCode, EvidenceJson: item.Evidence, VerifiedBy: item.VerifiedBy, OccurredAt: timestamp(item.OccurredAt), RequestId: item.RequestID})
	}
	return out
}

func safeIdentityAdminURL(raw string, allowedHosts []string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return false
	}
	localHTTP := strings.EqualFold(u.Scheme, "http") && (strings.EqualFold(u.Hostname(), "localhost") || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
	if (!strings.EqualFold(u.Scheme, "https") && !localHTTP) || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	for _, host := range allowedHosts {
		if strings.EqualFold(strings.TrimSpace(host), u.Hostname()) {
			return true
		}
	}
	return false
}

func portalCategoriesProto(items []portaldomain.Category) []*forgev1.PortalCategory {
	out := make([]*forgev1.PortalCategory, 0, len(items))
	for _, item := range items {
		out = append(out, portalCategoryProto(item))
	}
	return out
}
func portalCategoryProto(item portaldomain.Category) *forgev1.PortalCategory {
	return &forgev1.PortalCategory{Id: item.ID, OrganizationId: item.OrganizationID, CategoryKey: item.Key, Name: item.Name, Description: item.Description, Status: item.Status, SortOrder: int64(item.SortOrder), CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)}
}
func portalTagsProto(items []portaldomain.Tag) []*forgev1.PortalTag {
	out := make([]*forgev1.PortalTag, 0, len(items))
	for _, item := range items {
		out = append(out, portalTagProto(item))
	}
	return out
}
func portalTagProto(item portaldomain.Tag) *forgev1.PortalTag {
	return &forgev1.PortalTag{Id: item.ID, OrganizationId: item.OrganizationID, TagKey: item.Key, Name: item.Name, SortOrder: int64(item.SortOrder), CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)}
}
func portalPoliciesProto(items []portaldomain.AccessPolicy) []*forgev1.PortalAccessPolicy {
	out := make([]*forgev1.PortalAccessPolicy, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalAccessPolicy{Id: item.ID, ApplicationId: item.ApplicationID, PolicyType: item.Type, Value: item.Value, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)})
	}
	return out
}

func portalApplicationRolesProto(items []portaldomain.ApplicationRole) []*forgev1.PortalApplicationRole {
	out := make([]*forgev1.PortalApplicationRole, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalApplicationRole{Id: item.ID, ApplicationId: item.ApplicationID, RoleKey: item.Key, Name: item.Name, Description: item.Description, RiskLevel: item.RiskLevel, Status: item.Status, ConfigVersion: item.ConfigVersion, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)})
	}
	return out
}
