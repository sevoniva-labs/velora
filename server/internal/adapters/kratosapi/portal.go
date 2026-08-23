package kratosapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	approvalapp "github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	appportal "github.com/sevoniva-labs/velora/server/internal/app/portal"
	approvaldomain "github.com/sevoniva-labs/velora/server/internal/domain/approval"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
	"github.com/sevoniva-labs/velora/server/internal/platform/casdooradmin"
	"github.com/sevoniva-labs/velora/server/internal/platform/credentialhandoff"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpserver"
	"github.com/sevoniva-labs/velora/server/internal/platform/idempotency"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
	"github.com/sevoniva-labs/velora/server/internal/platform/provisioninghttp"
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
	handoff                   *credentialhandoff.Store
	provisioningRouter        *provisioninghttp.Router
}

func setNoStore(ctx context.Context) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "no-store")
		tr.ReplyHeader().Set("Pragma", "no-cache")
	}
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

func (s *PortalService) ConfigureCredentialHandoff(store *credentialhandoff.Store) {
	s.handoff = store
}

func (s *PortalService) ConfigureProvisioningRouter(router *provisioninghttp.Router) {
	s.provisioningRouter = router
}

// RunProviderReconciler continuously compares Velora's desired OIDC client
// configuration with Casdoor. It records only redacted operation evidence and
// never silently overwrites provider-side changes.
func (s *PortalService) RunProviderReconciler(ctx context.Context, interval time.Duration) {
	if s.casdoorAutomation == nil || !s.casdoorAutomation.Enabled() {
		return
	}
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	run := func() {
		items, err := s.portal.ListProviderReconciliationCandidates(ctx, 500)
		if err != nil {
			slog.Error("application provider reconciliation query failed", "error", err)
			return
		}
		for _, item := range items {
			if ctx.Err() != nil {
				return
			}
			principal := domain.Principal{OrganizationID: item.Application.OrganizationID}
			latest, latestErr := s.portal.LatestOnboardingOperation(ctx, principal, item.Application.ID)
			if latestErr != nil && !errors.Is(latestErr, appportal.ErrNotFound) {
				continue
			}
			s.reconcileProviderDrift(ctx, principal, item.Application, item.Binding, latest)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
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
			item, updateErr = s.portal.UpdateApplication(txCtx, principal, req.GetApplicationId(), repository.ApplicationInput{Name: req.GetName(), Description: req.GetDescription(), Icon: req.GetIcon(), CategoryID: req.GetCategoryId(), HomeURL: req.GetHomeUrl(), LaunchURL: req.GetLaunchUrl(), LaunchType: req.GetLaunchType(), Status: req.GetStatus(), SortOrder: int(req.GetSortOrder()), Featured: req.GetFeatured(), TagIDs: req.GetTagIds(), OwnerUserID: req.GetOwnerUserId(), OwnerDepartmentID: req.GetOwnerDepartmentId()})
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
	if _, err := requiredPrincipal(ctx); err != nil {
		return nil, err
	}
	return nil, kratoserrors.BadRequest("APPLICATION_DELETE_DISABLED", "application deletion is disabled; stop the application and retain its audit history")
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
	if _, err := requiredPrincipal(ctx); err != nil {
		return nil, err
	}
	return nil, kratoserrors.BadRequest("LEGACY_POLICY_MUTATION_DISABLED", "portal policies are read-only; manage application access grants instead")
}

func (s *PortalService) ListPortalApplicationAccessGrants(ctx context.Context, req *forgev1.ListPortalApplicationAccessGrantsRequest) (*forgev1.ListPortalApplicationAccessGrantsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.ListAccessGrants(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ListPortalApplicationAccessGrantsResponse{Grants: accessGrantsProto(items)}, nil
}

func (s *PortalService) PreviewPortalApplicationAccessGrants(ctx context.Context, req *forgev1.PreviewPortalApplicationAccessGrantsRequest) (*forgev1.PreviewPortalApplicationAccessGrantsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	impact, effective, err := s.portal.PreviewAccessGrants(ctx, principal, req.GetApplicationId(), accessGrantsDomain(req.GetGrants()))
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.PreviewPortalApplicationAccessGrantsResponse{Impact: accessImpactProto(impact), EffectiveAccess: effectiveAccessProto(effective)}, nil
}

func (s *PortalService) ReplacePortalApplicationAccessGrants(ctx context.Context, req *forgev1.ReplacePortalApplicationAccessGrantsRequest) (*forgev1.ReplacePortalApplicationAccessGrantsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	grants := accessGrantsDomain(req.GetGrants())
	impact, _, err := s.portal.PreviewAccessGrants(ctx, principal, req.GetApplicationId(), grants)
	if err != nil {
		return nil, serviceError(err)
	}
	payload, err := json.Marshal(map[string]any{"application_id": req.GetApplicationId(), "grants": accessGrantsApprovalPayload(req.GetGrants())})
	if err != nil {
		return nil, internalError(err)
	}
	response, err := s.idempotent(ctx, principal, "portal.application.access_grants.replace", req, func() proto.Message { return &forgev1.ReplacePortalApplicationAccessGrantsResponse{} }, func() (proto.Message, error) {
		var out []portaldomain.AccessGrant
		var applied repository.AccessImpactPreview
		event := newAuditEvent(ctx, principal, "portal.application.access_grants.replace", "portal_application", req.GetApplicationId(), map[string]any{"effective_users": impact.EffectiveUsers, "added_users": impact.AddedUsers, "revoked_users": impact.RevokedUsers, "role_changed_users": impact.RoleChangedUsers, "privileged_users": impact.PrivilegedUsers, "approval_id": req.GetApprovalId()})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			if impact.PrivilegedUsers > 0 || impact.ProvisioningTasks >= 50 {
				if s.approval == nil {
					return approvalapp.ErrApprovalRequired
				}
				if approvalErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), approvalapp.ExecutionInput{RequestType: "APPLICATION_ACCESS_CHANGE", Action: "portal.application.access_grants.replace", Resource: "portal_application", ResourceID: req.GetApplicationId(), PayloadJSON: string(payload)}); approvalErr != nil {
					return approvalErr
				}
			}
			var replaceErr error
			out, applied, replaceErr = s.portal.ReplaceAccessGrants(txCtx, principal, req.GetApplicationId(), grants)
			return replaceErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.ReplacePortalApplicationAccessGrantsResponse{Grants: accessGrantsProto(out), Impact: accessImpactProto(applied)}, nil
	})
	if err != nil {
		return nil, err
	}
	return response.(*forgev1.ReplacePortalApplicationAccessGrantsResponse), nil
}

func accessGrantsApprovalPayload(items []*forgev1.PortalApplicationAccessGrant) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		grant := map[string]any{
			"id": item.GetId(), "application_id": item.GetApplicationId(), "subject_type": item.GetSubjectType(),
			"subject_id": item.GetSubjectId(), "include_descendants": item.GetIncludeDescendants(), "effect": item.GetEffect(),
			"roles": item.GetRoles(), "status": item.GetStatus(), "reason": item.GetReason(), "version": item.GetVersion(),
		}
		if item.GetValidFrom() != nil {
			grant["valid_from"] = item.GetValidFrom().AsTime().UTC().Format(time.RFC3339Nano)
		}
		if item.GetValidUntil() != nil {
			grant["valid_until"] = item.GetValidUntil().AsTime().UTC().Format(time.RFC3339Nano)
		}
		out = append(out, grant)
	}
	return out
}

func (s *PortalService) ListPortalApplicationEffectiveAccess(ctx context.Context, req *forgev1.ListPortalApplicationEffectiveAccessRequest) (*forgev1.ListPortalApplicationEffectiveAccessResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.ListEffectiveAccess(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ListPortalApplicationEffectiveAccessResponse{EffectiveAccess: effectiveAccessProto(items)}, nil
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

func (s *PortalService) GetPortalApplicationProvisioningTarget(ctx context.Context, req *forgev1.GetPortalApplicationProvisioningTargetRequest) (*forgev1.GetPortalApplicationProvisioningTargetResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.portal.GetProvisioningTarget(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.GetPortalApplicationProvisioningTargetResponse{Target: provisioningTargetProto(target)}, nil
}

func (s *PortalService) UpsertPortalApplicationProvisioningTarget(ctx context.Context, req *forgev1.UpsertPortalApplicationProvisioningTargetRequest) (*forgev1.UpsertPortalApplicationProvisioningTargetResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	response, err := s.idempotentWith(ctx, principal, "portal.application.provisioning.upsert", req, func() proto.Message { return &forgev1.UpsertPortalApplicationProvisioningTargetResponse{} }, func() (proto.Message, error) {
		var target portaldomain.ProvisioningTarget
		var oneTimeSecret string
		event := newAuditEvent(ctx, principal, "portal.application.provisioning.upsert", "portal_application", req.GetApplicationId(), map[string]any{"rotate_secret": req.GetRotateSecret()})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var updateErr error
			target, oneTimeSecret, updateErr = s.portal.UpsertProvisioningTarget(txCtx, principal, req.GetApplicationId(), req.GetEndpointUrl(), req.GetRotateSecret(), req.GetExpectedConfigVersion())
			return updateErr
		}); err != nil {
			return nil, serviceError(err)
		}
		return &forgev1.UpsertPortalApplicationProvisioningTargetResponse{Target: provisioningTargetProto(target), OneTimeProvisioningSecret: oneTimeSecret}, nil
	}, func(message proto.Message) proto.Message {
		cached := proto.Clone(message).(*forgev1.UpsertPortalApplicationProvisioningTargetResponse)
		cached.OneTimeProvisioningSecret = ""
		return cached
	})
	if err != nil {
		return nil, err
	}
	reply := response.(*forgev1.UpsertPortalApplicationProvisioningTargetResponse)
	if strings.EqualFold(strings.TrimSpace(req.GetCredentialDeliveryMode()), "CLI") {
		reply.OneTimeProvisioningSecret = ""
	}
	if reply.GetOneTimeProvisioningSecret() != "" {
		setNoStore(ctx)
	}
	return reply, nil
}

func (s *PortalService) RetryPortalApplicationProvisioning(ctx context.Context, req *forgev1.RetryPortalApplicationProvisioningRequest) (*forgev1.RetryPortalApplicationProvisioningResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var retried int
	var target portaldomain.ProvisioningTarget
	event := newAuditEvent(ctx, principal, "portal.application.provisioning.retry", "portal_application", req.GetApplicationId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var retryErr error
		retried, target, retryErr = s.portal.RetryProvisioning(txCtx, principal, req.GetApplicationId())
		event.Details = map[string]any{"retried_messages": retried}
		return retryErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RetryPortalApplicationProvisioningResponse{RetriedMessages: int32(retried), Target: provisioningTargetProto(target)}, nil // #nosec G115 -- retry count is bounded by application outbox volume.
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
	roles, err := s.portal.ListApplicationRoles(ctx, principal, app.ID)
	if err != nil {
		return nil, serviceError(err)
	}
	target, targetErr := s.portal.GetProvisioningTarget(ctx, principal, app.ID)
	if targetErr != nil && !errors.Is(targetErr, appportal.ErrNotFound) {
		return nil, serviceError(targetErr)
	}
	checks, err := s.portal.ListOnboardingChecks(ctx, principal, app.ID, app.ConfigVersion)
	if err != nil {
		return nil, serviceError(err)
	}
	latestOperation, operationErr := s.portal.LatestOnboardingOperation(ctx, principal, app.ID)
	if operationErr != nil && !errors.Is(operationErr, appportal.ErrNotFound) {
		return nil, serviceError(operationErr)
	}
	if s.casdoorAutomation != nil && s.casdoorAutomation.Enabled() && binding.ID != "" {
		latestOperation = s.reconcileProviderDrift(ctx, principal, app, binding, latestOperation)
	}
	status, nextAction, blockers, canPublish := onboardingState(app, binding, target, checks)
	if latestOperation.Status == portaldomain.OperationFailed || latestOperation.Status == portaldomain.OperationActionRequired {
		status, canPublish = "ACTION_REQUIRED", false
		blockers = append(blockers, "身份提供方配置需要处理："+latestOperation.ErrorCode)
		nextAction = "修复身份提供方操作或配置漂移后重新检查。"
	}
	response := &forgev1.GetApplicationOnboardingResponse{Application: portalApplicationProto(app), Binding: identityBindingProto(binding), Verifications: verificationsProto(verifications), CanPublish: canPublish, OnboardingStatus: status, NextAction: nextAction, Blockers: blockers, Roles: portalApplicationRolesProto(roles), OnboardingChecks: onboardingChecksProto(checks)}
	if target.ID != "" {
		response.ProvisioningTarget = provisioningTargetProto(target)
	}
	if latestOperation.ID != "" {
		response.LatestOperation = onboardingOperationProto(latestOperation)
	}
	return response, nil
}

func (s *PortalService) reconcileProviderDrift(ctx context.Context, principal domain.Principal, app portaldomain.Application, binding portaldomain.IdentityBinding, latest portaldomain.OnboardingOperation) portaldomain.OnboardingOperation {
	providerApp, found, err := s.casdoorAutomation.GetApplication(ctx, binding.ProviderApplicationRef)
	status, code, summary := portaldomain.OperationSucceeded, "", `{"drift":false}`
	if err != nil {
		status, code, summary = portaldomain.OperationFailed, "PROVIDER_RECONCILIATION_FAILED", `{"provider_reachable":false}`
	} else if providerConfigurationDrift(found, providerApp, binding) {
		status, code, summary = portaldomain.OperationActionRequired, "PROVIDER_CONFIGURATION_DRIFT", `{"drift":true}`
	}
	key := fmt.Sprintf("RECONCILE:%s:%d:%s", app.ID, app.ConfigVersion, code)
	if latest.OperationType == "RECONCILE_PROVIDER" && latest.IdempotencyKey == key && latest.Status == status {
		return latest
	}
	operation, beginErr := s.portal.BeginOnboardingOperation(ctx, principal, app.ID, "RECONCILE_PROVIDER", app.ConfigVersion, key)
	if beginErr != nil {
		return latest
	}
	var retryAt *time.Time
	if status == portaldomain.OperationFailed {
		next := time.Now().UTC().Add(5 * time.Minute)
		retryAt = &next
	}
	if completeErr := s.portal.CompleteOnboardingOperation(ctx, operation.ID, status, code, summary, retryAt); completeErr != nil {
		return operation
	}
	operation.Status, operation.ErrorCode, operation.ResultSummaryJSON, operation.NextRetryAt = status, code, summary, retryAt
	return operation
}

func providerConfigurationDrift(found bool, providerApp casdooradmin.Application, binding portaldomain.IdentityBinding) bool {
	// Casdoor versions before the application-scope schema do not expose
	// Scopes. In that case discovery/token checks remain authoritative; when
	// scopes are exposed they are included in drift detection.
	return !found || providerApp.ClientID != binding.PublicClientID || !equalStrings(providerApp.RedirectURIs, binding.RedirectURIs) || (len(providerApp.Scopes) > 0 && !equalStrings(providerApp.Scopes, binding.Scopes)) || !providerApp.Enabled
}

func equalStrings(left, right []string) bool {
	a, b := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *PortalService) PrepareApplicationCredentialApproval(ctx context.Context, req *forgev1.PrepareApplicationCredentialApprovalRequest) (*forgev1.PrepareApplicationCredentialApprovalResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.approval == nil || s.casdoorAutomation == nil || !s.casdoorAutomation.Enabled() {
		return nil, kratoserrors.ServiceUnavailable("CREDENTIAL_AUTOMATION_UNAVAILABLE", "credential automation is unavailable")
	}
	app, _, _, err := s.portal.GetApplicationOnboarding(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	providerRef, clientID := generatedOIDCIdentifiers(principal.OrganizationID, app.Code)
	scopes := []string{"openid", "profile", "email"}
	payload := credentialApprovalPayload(providerRef, clientID, s.identityIssuer, req.GetRedirectUris(), scopes)
	if err := (portaldomain.IdentityBindingInput{ProviderKey: portaldomain.IdentityProviderCasdoor, Protocol: portaldomain.ProtocolOIDC, ProviderApplicationRef: providerRef, PublicClientID: clientID, Issuer: s.identityIssuer, RedirectURIs: req.GetRedirectUris(), Scopes: scopes}).Validate(); err != nil {
		return nil, serviceError(err)
	}
	encoded, _ := json.Marshal(payload)
	if existing, found := s.findCredentialApproval(ctx, principal, app.ID, string(encoded)); found {
		return credentialApprovalResponse(existing, providerRef, clientID, s.identityIssuer, scopes, s.approverName(ctx, principal.OrganizationID, existing)), nil
	}
	var approverID string
	err = s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id JOIN roles r ON r.id=ur.role_id AND r.organization_id=u.organization_id WHERE u.organization_id=? AND u.id<>? AND u.status='ACTIVE' AND r.role_key IN ('system_admin','security_admin') ORDER BY CASE WHEN r.role_key='security_admin' THEN 0 ELSE 1 END,u.created_at LIMIT 1`), principal.OrganizationID, principal.UserID).Scan(&approverID)
	if err != nil {
		return nil, kratoserrors.Conflict("APPROVER_UNAVAILABLE", "no independent security approver is available")
	}
	var created approvaldomain.Request
	event := newAuditEvent(ctx, principal, "portal.application.credential_approval.create", "portal_application", app.ID, map[string]any{"approver_id": approverID})
	if err := s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.approval.Create(txCtx, principal, approvalapp.CreateInput{RequestType: "CASDOOR_APPLICATION", Action: "UPSERT", Resource: "portal_application", ResourceID: app.ID, Summary: "签发 " + app.Name + " 的统一登录与账号同步凭据", PayloadJSON: string(encoded), Mode: approvaldomain.ModeAny, RequiredApprovals: 1, ApproverIDs: []string{approverID}, ExpiresIn: 24 * time.Hour})
		return createErr
	}); err != nil {
		return nil, approvalServiceError(err)
	}
	return credentialApprovalResponse(created, providerRef, clientID, s.identityIssuer, scopes, s.approverName(ctx, principal.OrganizationID, created)), nil
}

func generatedOIDCIdentifiers(organizationID, applicationCode string) (string, string) {
	ref := "velora-" + strings.ToLower(strings.TrimSpace(applicationCode))
	digest := sha256.Sum256([]byte(organizationID + ":" + applicationCode))
	return ref, "vlr_" + fmt.Sprintf("%x", digest[:12])
}

func credentialApprovalPayload(providerRef, clientID, issuer string, redirects, scopes []string) map[string]any {
	return map[string]any{"provider": portaldomain.IdentityProviderCasdoor, "protocol": portaldomain.ProtocolOIDC, "provider_application_ref": providerRef, "public_client_id": clientID, "issuer": issuer, "redirect_uris": redirects, "scopes": scopes}
}

func (s *PortalService) findCredentialApproval(ctx context.Context, principal domain.Principal, applicationID, payloadJSON string) (approvaldomain.Request, bool) {
	items, err := s.approval.List(ctx, principal)
	if err != nil {
		return approvaldomain.Request{}, false
	}
	for _, item := range items {
		if item.RequestType == "CASDOOR_APPLICATION" && item.Action == "UPSERT" && item.ResourceID == applicationID && item.PayloadJSON == payloadJSON && (item.Status == approvaldomain.StatusPending || item.Status == approvaldomain.StatusApproved) {
			return item, true
		}
	}
	return approvaldomain.Request{}, false
}

func (s *PortalService) approverName(ctx context.Context, organizationID string, item approvaldomain.Request) string {
	if len(item.Tasks) == 0 {
		return "安全审批人"
	}
	var name string
	if err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT display_name FROM users WHERE organization_id=? AND id=?`), organizationID, item.Tasks[0].AssigneeID).Scan(&name); err != nil || strings.TrimSpace(name) == "" {
		return "安全审批人"
	}
	return name
}

func credentialApprovalResponse(item approvaldomain.Request, providerRef, clientID, issuer string, scopes []string, approverName string) *forgev1.PrepareApplicationCredentialApprovalResponse {
	next := "等待 " + approverName + " 审批，批准后在本页继续生成接入配置。"
	if item.Status == approvaldomain.StatusApproved {
		next = "审批已通过，可以生成接入配置。"
	}
	return &forgev1.PrepareApplicationCredentialApprovalResponse{ApprovalStatus: item.Status, ApproverName: approverName, NextAction: next, ProviderApplicationRef: providerRef, PublicClientId: clientID, Issuer: issuer, Scopes: scopes}
}

func onboardingState(app portaldomain.Application, binding portaldomain.IdentityBinding, target portaldomain.ProvisioningTarget, checks []portaldomain.OnboardingCheck) (string, string, []string, bool) {
	if app.LifecycleStatus == portaldomain.LifecyclePublished && app.Status == portaldomain.StatusEnabled {
		return "PUBLISHED", "应用已发布；持续关注登录和账号同步健康状态。", nil, true
	}
	if app.Status == portaldomain.StatusDisabled && app.LifecycleStatus == portaldomain.LifecycleDisabled {
		return "SUSPENDED", "恢复应用后重新执行受影响的检查。", []string{"应用已停用"}, false
	}
	if strings.EqualFold(app.LaunchType, "URL") {
		if len(app.Policies) == 0 {
			return "DRAFT", "配置访问范围。", []string{"访问范围未配置（默认拒绝）"}, false
		}
		return "VERIFIED", "提交并发布应用。", nil, true
	}
	blockers := make([]string, 0, 3)
	if binding.ID == "" {
		blockers = append(blockers, "统一登录配置尚未生成")
	}
	if target.ID == "" {
		blockers = append(blockers, "账号同步目标尚未配置")
	}
	if len(app.Policies) == 0 {
		blockers = append(blockers, "访问范围未配置（默认拒绝）")
	}
	if len(blockers) > 0 {
		return "DRAFT", "完成登录、账号同步和访问范围配置。", blockers, false
	}
	if binding.VerificationStatus != portaldomain.VerificationPassed {
		return "WAITING_FOR_DEPLOYMENT", "部署目标应用配置后执行身份验证。", []string{"OIDC 配置尚未验证"}, false
	}
	if target.DeliveryStatus != "HEALTHY" {
		return "ACTION_REQUIRED", "完成账号同步 challenge 或测试投递。", []string{"账号同步链路尚未验证"}, false
	}
	if !portaldomain.OnboardingChecksPassed(checks) {
		return "ACTION_REQUIRED", "运行当前配置版本的全部自动检查。", []string{"发布门禁检查尚未全部通过"}, false
	}
	return "VERIFIED", "提交并发布应用。", nil, true
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
	response, err := s.idempotentWith(ctx, principal, "iam.integration.update", req, func() proto.Message { return &forgev1.UpsertApplicationIdentityBindingResponse{} }, func() (proto.Message, error) {
		var automationScopes []string
		approvalID := strings.TrimSpace(req.GetApprovalId())
		if automationEnabled {
			var scopeErr error
			automationScopes, scopeErr = portaldomain.NormalizeOIDCScopes(req.GetScopes())
			if scopeErr != nil {
				return nil, serviceError(scopeErr)
			}
			payload := map[string]any{
				"provider":                 portaldomain.IdentityProviderCasdoor,
				"protocol":                 req.GetProtocol(),
				"provider_application_ref": req.GetProviderApplicationRef(),
				"public_client_id":         req.GetPublicClientId(),
				"issuer":                   req.GetIssuer(),
				"redirect_uris":            req.GetRedirectUris(),
				"scopes":                   automationScopes,
			}
			if approvalID == "" {
				encoded, _ := json.Marshal(payload)
				if item, found := s.findCredentialApproval(ctx, principal, req.GetApplicationId(), string(encoded)); found && item.Status == approvaldomain.StatusApproved {
					approvalID = item.ID
				}
			}
			if err := s.authorizeCasdoorAutomation(ctx, principal, approvalID, "UPSERT", req.GetApplicationId(), payload); err != nil {
				return nil, serviceError(err)
			}
		}
		var binding portaldomain.IdentityBinding
		var app portaldomain.Application
		var oneTimeClientSecret string
		var enrollmentToken string
		var enrollmentExpiresAt time.Time
		var onboardingOperation portaldomain.OnboardingOperation
		operationKey := ""
		if automationEnabled {
			digest := sha256.Sum256([]byte(strings.Join([]string{req.GetProviderApplicationRef(), req.GetPublicClientId(), strings.Join(req.GetRedirectUris(), "\x00"), strings.Join(automationScopes, "\x00")}, "\x01")))
			operationKey = fmt.Sprintf("UPSERT_PROVIDER:%s:%x", req.GetApplicationId(), digest[:16])
		}
		event := newAuditEvent(ctx, principal, "iam.integration.update", "portal_application", req.GetApplicationId(), map[string]any{"protocol": req.GetProtocol(), "provider": portaldomain.IdentityProviderCasdoor})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			binding, app, operationErr = s.portal.UpsertApplicationIdentityBinding(txCtx, principal, req.GetApplicationId(), portaldomain.IdentityBindingInput{ProviderKey: req.GetProviderKey(), Protocol: req.GetProtocol(), ProviderApplicationRef: req.GetProviderApplicationRef(), PublicClientID: req.GetPublicClientId(), Issuer: req.GetIssuer(), RedirectURIs: req.GetRedirectUris(), Scopes: req.GetScopes()}, req.GetExpectedConfigVersion())
			if operationErr != nil || !automationEnabled {
				return operationErr
			}
			onboardingOperation, operationErr = s.portal.BeginOnboardingOperation(txCtx, principal, app.ID, "UPSERT_PROVIDER", app.ConfigVersion, operationKey)
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		// Keep the Velora binding in VERIFICATION_PENDING when the external call
		// fails. A later retry is safe because the Casdoor provider upsert is
		// idempotent and the local binding is already auditable and recoverable.
		if automationEnabled {
			application, _, automationErr := s.casdoorAutomation.UpsertApplication(ctx, casdooradmin.UpsertInput{Name: req.GetProviderApplicationRef(), Organization: principal.OrganizationID, DisplayName: req.GetProviderApplicationRef(), ClientID: req.GetPublicClientId(), RedirectURIs: req.GetRedirectUris(), GrantTypes: []string{"authorization_code"}, Scopes: automationScopes, ApprovalID: approvalID, RotateSecret: strings.TrimSpace(req.GetCredentialDeliveryMode()) != ""})
			if automationErr != nil {
				next := time.Now().UTC().Add(time.Minute)
				_ = s.portal.CompleteOnboardingOperation(ctx, onboardingOperation.ID, portaldomain.OperationFailed, "PROVIDER_UPSERT_FAILED", `{"provider":"casdoor"}`, &next)
				return nil, serviceError(automationErr)
			}
			oneTimeClientSecret = application.TakeOneTimeClientSecret()
		}
		if strings.EqualFold(strings.TrimSpace(req.GetCredentialDeliveryMode()), "CLI") {
			if oneTimeClientSecret == "" || s.handoff == nil {
				if onboardingOperation.ID != "" {
					next := time.Now().UTC().Add(time.Minute)
					_ = s.portal.CompleteOnboardingOperation(ctx, onboardingOperation.ID, portaldomain.OperationFailed, "ENROLLMENT_CREDENTIAL_UNAVAILABLE", `{"provider":"casdoor"}`, &next)
				}
				return nil, kratoserrors.Conflict("ENROLLMENT_CREDENTIAL_UNAVAILABLE", "new credentials must be generated before CLI enrollment can be issued")
			}
			application, target, provisioningSecret, handoffErr := s.portal.ProvisioningCredentialForHandoff(ctx, principal, req.GetApplicationId())
			if handoffErr != nil {
				return nil, serviceError(handoffErr)
			}
			enrollmentToken, enrollmentExpiresAt, handoffErr = s.handoff.Issue(ctx, credentialhandoff.Bundle{ApplicationCode: application.Code, Issuer: binding.Issuer, ClientID: binding.PublicClientID, ClientSecret: oneTimeClientSecret, RedirectURIs: binding.RedirectURIs, Scopes: binding.Scopes, ProvisioningEndpoint: target.EndpointURL, ProvisioningSecret: provisioningSecret, ProvisioningKeyVersion: target.ActiveKeyVersion, ProvisioningFingerprint: target.SecretFingerprint})
			if handoffErr != nil {
				if onboardingOperation.ID != "" {
					next := time.Now().UTC().Add(time.Minute)
					_ = s.portal.CompleteOnboardingOperation(ctx, onboardingOperation.ID, portaldomain.OperationFailed, "ENROLLMENT_ISSUE_FAILED", `{"provider":"casdoor"}`, &next)
				}
				return nil, internalError(handoffErr)
			}
			oneTimeClientSecret = ""
		}
		if onboardingOperation.ID != "" {
			if completeErr := s.portal.CompleteOnboardingOperation(ctx, onboardingOperation.ID, portaldomain.OperationSucceeded, "", `{"provider":"casdoor","configured":true}`, nil); completeErr != nil {
				return nil, internalError(completeErr)
			}
		}
		return &forgev1.UpsertApplicationIdentityBindingResponse{Binding: identityBindingProto(binding), Application: portalApplicationProto(app), OneTimeClientSecret: oneTimeClientSecret, EnrollmentToken: enrollmentToken, EnrollmentExpiresAt: optionalTimestamp(optionalTime(enrollmentExpiresAt))}, nil
	}, func(message proto.Message) proto.Message {
		cached := proto.Clone(message).(*forgev1.UpsertApplicationIdentityBindingResponse)
		cached.OneTimeClientSecret = ""
		cached.EnrollmentToken = ""
		cached.EnrollmentExpiresAt = nil
		return cached
	})
	if err != nil {
		return nil, err
	}
	reply := response.(*forgev1.UpsertApplicationIdentityBindingResponse)
	if reply.GetOneTimeClientSecret() != "" || reply.GetEnrollmentToken() != "" {
		setNoStore(ctx)
	}
	return reply, nil
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func (s *PortalService) ConsumeApplicationEnrollment(ctx context.Context, req *forgev1.ConsumeApplicationEnrollmentRequest) (*forgev1.ConsumeApplicationEnrollmentResponse, error) {
	if s.handoff == nil {
		return nil, kratoserrors.ServiceUnavailable("ENROLLMENT_UNAVAILABLE", "credential enrollment is unavailable")
	}
	bundle, err := s.handoff.Consume(ctx, req.GetEnrollmentToken())
	if err != nil {
		return nil, kratoserrors.Unauthorized("ENROLLMENT_TOKEN_INVALID", "enrollment token is invalid or expired")
	}
	setNoStore(ctx)
	return &forgev1.ConsumeApplicationEnrollmentResponse{ApplicationCode: bundle.ApplicationCode, Issuer: bundle.Issuer, ClientId: bundle.ClientID, ClientSecret: bundle.ClientSecret, RedirectUris: bundle.RedirectURIs, Scopes: bundle.Scopes, ProvisioningEndpoint: bundle.ProvisioningEndpoint, ProvisioningSecret: bundle.ProvisioningSecret, ProvisioningKeyVersion: bundle.ProvisioningKeyVersion, ProvisioningFingerprint: bundle.ProvisioningFingerprint}, nil
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

func (s *PortalService) RunApplicationOnboardingChecks(ctx context.Context, req *forgev1.RunApplicationOnboardingChecksRequest) (*forgev1.RunApplicationOnboardingChecksResponse, error) {
	if err := s.identityOnboardingRequired(); err != nil {
		return nil, err
	}
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.provisioningRouter == nil {
		return nil, kratoserrors.ServiceUnavailable("PROVISIONING_CHECK_UNAVAILABLE", "provisioning check service is unavailable")
	}
	app, binding, _, err := s.portal.GetApplicationOnboarding(ctx, principal, req.GetApplicationId())
	if err != nil {
		return nil, serviceError(err)
	}
	if req.GetExpectedConfigVersion() != app.ConfigVersion {
		return nil, serviceError(portaldomain.ErrOptimisticConflict)
	}
	checks := make([]portaldomain.OnboardingCheck, 0, 5)
	add := func(checkType string, passed bool, code string, evidence map[string]any) {
		result := "PASSED"
		if !passed {
			result = "FAILED"
		}
		raw, _ := json.Marshal(evidence)
		checks = append(checks, portaldomain.OnboardingCheck{CheckType: checkType, Result: result, ErrorCode: code, EvidenceJSON: string(raw)})
	}
	accessGrants, accessErr := s.portal.ListAccessGrants(ctx, principal, app.ID)
	hasAccess := accessErr == nil && len(accessGrants) > 0 || len(app.Policies) > 0
	add("access_policy", hasAccess, valueIf(!hasAccess, "ACCESS_POLICY_REQUIRED"), map[string]any{"grant_count": len(accessGrants)})
	identityPassed := false
	if binding.ID != "" {
		_, updatedApp, _, passed, verifyErr := s.portal.VerifyApplicationIdentity(ctx, principal, app.ID, binding.ConfigVersion)
		identityPassed = verifyErr == nil && passed
		if verifyErr == nil {
			app = updatedApp
		}
		add("oidc_discovery", identityPassed, valueIf(!identityPassed, "OIDC_VERIFICATION_FAILED"), map[string]any{"issuer": binding.Issuer})
	} else {
		add("oidc_discovery", false, "IDENTITY_BINDING_REQUIRED", map[string]any{})
	}
	challengeID := uuid.NewString()
	topic := provisioninghttp.ProvisioningTopicPrefix + app.Code
	body := func(eventID string, version int64) []byte {
		raw, _ := json.Marshal(map[string]any{"schema_version": "1.0", "event_id": eventID, "event_type": "integration.challenge", "aggregate_version": version, "occurred_at": time.Now().UTC().Format(time.RFC3339Nano), "source": "velora", "challenge": map[string]any{"application_code": app.Code, "challenge_id": challengeID}})
		return raw
	}
	eventID := uuid.NewString()
	appliedBody := body(eventID, 2)
	_, appliedStatus, appliedErr := s.provisioningRouter.PublishWithStatus(ctx, messaging.Message{ID: eventID, OrganizationID: principal.OrganizationID, Topic: topic, Type: "integration.challenge", Body: appliedBody})
	add("provisioning_challenge", appliedErr == nil && appliedStatus == "APPLIED", valueIf(appliedErr != nil || appliedStatus != "APPLIED", "PROVISIONING_CHALLENGE_FAILED"), map[string]any{"status": appliedStatus})
	_, duplicateStatus, duplicateErr := s.provisioningRouter.PublishWithStatus(ctx, messaging.Message{ID: eventID, OrganizationID: principal.OrganizationID, Topic: topic, Type: "integration.challenge", Body: appliedBody})
	add("provisioning_duplicate", duplicateErr == nil && duplicateStatus == "DUPLICATE", valueIf(duplicateErr != nil || duplicateStatus != "DUPLICATE", "PROVISIONING_DUPLICATE_FAILED"), map[string]any{"status": duplicateStatus})
	staleID := uuid.NewString()
	_, staleStatus, staleErr := s.provisioningRouter.PublishWithStatus(ctx, messaging.Message{ID: staleID, OrganizationID: principal.OrganizationID, Topic: topic, Type: "integration.challenge", Body: body(staleID, 1)})
	add("provisioning_stale", staleErr == nil && staleStatus == "STALE", valueIf(staleErr != nil || staleStatus != "STALE", "PROVISIONING_STALE_FAILED"), map[string]any{"status": staleStatus})
	recorded, err := s.portal.RecordOnboardingChecks(ctx, principal, app.ID, app.ConfigVersion, checks)
	if err != nil {
		return nil, serviceError(err)
	}
	passed := true
	for _, check := range recorded {
		if check.Result != "PASSED" {
			passed = false
			break
		}
	}
	return &forgev1.RunApplicationOnboardingChecksResponse{Checks: onboardingChecksProto(recorded), Passed: passed}, nil
}

func valueIf(condition bool, value string) string {
	if condition {
		return value
	}
	return ""
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
		var onboardingOperation portaldomain.OnboardingOperation
		event := newAuditEvent(ctx, principal, "portal.application.disable", "portal_application", req.GetApplicationId(), map[string]any{"approval_id": req.GetApprovalId(), "casdoor_automation": automationEnabled})
		if err := s.audited(ctx, event, func(txCtx context.Context) error {
			var operationErr error
			app, operationErr = s.portal.DisableApplication(txCtx, principal, req.GetApplicationId(), req.GetExpectedConfigVersion())
			if operationErr != nil || !automationEnabled {
				return operationErr
			}
			key := fmt.Sprintf("DISABLE_PROVIDER:%s:%d", app.ID, app.ConfigVersion)
			onboardingOperation, operationErr = s.portal.BeginOnboardingOperation(txCtx, principal, app.ID, "DISABLE_PROVIDER", app.ConfigVersion, key)
			return operationErr
		}); err != nil {
			return nil, serviceError(err)
		}
		if automationEnabled {
			if err := s.casdoorAutomation.DisableApplication(ctx, binding.ProviderApplicationRef, req.GetApprovalId()); err != nil {
				next := time.Now().UTC().Add(time.Minute)
				_ = s.portal.CompleteOnboardingOperation(ctx, onboardingOperation.ID, portaldomain.OperationFailed, "PROVIDER_DISABLE_FAILED", `{"provider":"casdoor"}`, &next)
				return nil, serviceError(err)
			}
			if err := s.portal.CompleteOnboardingOperation(ctx, onboardingOperation.ID, portaldomain.OperationSucceeded, "", `{"provider":"casdoor","disabled":true}`, nil); err != nil {
				return nil, internalError(err)
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
	return repository.ApplicationInput{Code: req.GetCode(), Name: req.GetName(), Description: req.GetDescription(), Icon: req.GetIcon(), CategoryID: req.GetCategoryId(), HomeURL: req.GetHomeUrl(), LaunchURL: req.GetLaunchUrl(), LaunchType: req.GetLaunchType(), Status: req.GetStatus(), SortOrder: int(req.GetSortOrder()), Featured: req.GetFeatured(), TagIDs: req.GetTagIds(), OwnerUserID: req.GetOwnerUserId(), OwnerDepartmentID: req.GetOwnerDepartmentId()}
}

func portalApplicationsProto(items []portaldomain.Application) []*forgev1.PortalApplication {
	out := make([]*forgev1.PortalApplication, 0, len(items))
	for _, item := range items {
		out = append(out, portalApplicationProto(item))
	}
	return out
}

func portalApplicationProto(item portaldomain.Application) *forgev1.PortalApplication {
	return &forgev1.PortalApplication{Id: item.ID, OrganizationId: item.OrganizationID, Code: item.Code, Name: item.Name, Description: item.Description, Icon: item.Icon, CategoryId: item.CategoryID, CategoryName: item.CategoryName, HomeUrl: item.HomeURL, LaunchUrl: item.LaunchURL, LaunchType: item.LaunchType, Status: item.Status, SortOrder: int64(item.SortOrder), Featured: item.Featured, Favorite: item.Favorite, VisitCount: item.VisitCount, Tags: portalTagsProto(item.Tags), Policies: portalPoliciesProto(item.Policies), CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt), LifecycleStatus: item.LifecycleStatus, ConfigVersion: item.ConfigVersion, PublishedAt: optionalTimestamp(item.PublishedAt), PublishedBy: item.PublishedBy, OwnerUserId: item.OwnerUserID, OwnerUserName: item.OwnerUserName, OwnerDepartmentId: item.OwnerDepartmentID, OwnerDepartmentName: item.OwnerDepartmentName}
}

func identityBindingProto(item portaldomain.IdentityBinding) *forgev1.PortalIdentityBinding {
	if item.ID == "" {
		return nil
	}
	return &forgev1.PortalIdentityBinding{Id: item.ID, OrganizationId: item.OrganizationID, ApplicationId: item.ApplicationID, ProviderKey: item.ProviderKey, Protocol: item.Protocol, ProviderApplicationRef: item.ProviderApplicationRef, PublicClientId: item.PublicClientID, Issuer: item.Issuer, RedirectUris: item.RedirectURIs, Scopes: item.Scopes, ConfigurationStatus: item.ConfigurationStatus, VerificationStatus: item.VerificationStatus, VerifiedAt: optionalTimestamp(item.VerifiedAt), VerifiedBy: item.VerifiedBy, VerificationError: item.VerificationError, ConfigVersion: item.ConfigVersion, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)}
}

func onboardingOperationProto(item portaldomain.OnboardingOperation) *forgev1.PortalApplicationOnboardingOperation {
	return &forgev1.PortalApplicationOnboardingOperation{Id: item.ID, OperationType: item.OperationType, DesiredVersion: item.DesiredVersion, Status: item.Status, AttemptCount: int32(item.AttemptCount), ErrorCode: item.ErrorCode, NextRetryAt: optionalTimestamp(item.NextRetryAt), UpdatedAt: timestamp(item.UpdatedAt), CompletedAt: optionalTimestamp(item.CompletedAt)}
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

func accessGrantsDomain(items []*forgev1.PortalApplicationAccessGrant) []portaldomain.AccessGrant {
	out := make([]portaldomain.AccessGrant, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, portaldomain.AccessGrant{ID: item.GetId(), ApplicationID: item.GetApplicationId(), SubjectType: item.GetSubjectType(), SubjectID: item.GetSubjectId(), IncludeDescendants: item.GetIncludeDescendants(), Effect: item.GetEffect(), Roles: append([]string(nil), item.GetRoles()...), ValidFrom: protoOptionalTime(item.GetValidFrom()), ValidUntil: protoOptionalTime(item.GetValidUntil()), Status: item.GetStatus(), Reason: item.GetReason(), Version: item.GetVersion()})
	}
	return out
}

func accessGrantsProto(items []portaldomain.AccessGrant) []*forgev1.PortalApplicationAccessGrant {
	out := make([]*forgev1.PortalApplicationAccessGrant, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalApplicationAccessGrant{Id: item.ID, ApplicationId: item.ApplicationID, SubjectType: item.SubjectType, SubjectId: item.SubjectID, SubjectName: item.SubjectName, IncludeDescendants: item.IncludeDescendants, Effect: item.Effect, Roles: item.Roles, ValidFrom: optionalTimestamp(item.ValidFrom), ValidUntil: optionalTimestamp(item.ValidUntil), Status: item.Status, Reason: item.Reason, Version: item.Version, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)})
	}
	return out
}

func effectiveAccessProto(items []portaldomain.EffectiveAccess) []*forgev1.PortalApplicationEffectiveAccess {
	out := make([]*forgev1.PortalApplicationEffectiveAccess, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalApplicationEffectiveAccess{UserId: item.UserID, LoginName: item.LoginName, DisplayName: item.DisplayName, Roles: item.Roles, SourceGrantIds: item.SourceGrantIDs})
	}
	return out
}

func accessImpactProto(item repository.AccessImpactPreview) *forgev1.PortalApplicationAccessImpact {
	return &forgev1.PortalApplicationAccessImpact{EffectiveUsers: item.EffectiveUsers, AddedUsers: item.AddedUsers, RevokedUsers: item.RevokedUsers, RoleChangedUsers: item.RoleChangedUsers, PrivilegedUsers: item.PrivilegedUsers, ProvisioningTasks: item.ProvisioningTasks}
}

func protoOptionalTime(value interface {
	AsTime() time.Time
	IsValid() bool
}) *time.Time {
	if value == nil || !value.IsValid() {
		return nil
	}
	result := value.AsTime().UTC()
	return &result
}

func portalApplicationRolesProto(items []portaldomain.ApplicationRole) []*forgev1.PortalApplicationRole {
	out := make([]*forgev1.PortalApplicationRole, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalApplicationRole{Id: item.ID, ApplicationId: item.ApplicationID, RoleKey: item.Key, Name: item.Name, Description: item.Description, RiskLevel: item.RiskLevel, Status: item.Status, ConfigVersion: item.ConfigVersion, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)})
	}
	return out
}

func provisioningTargetProto(item portaldomain.ProvisioningTarget) *forgev1.PortalApplicationProvisioningTarget {
	return &forgev1.PortalApplicationProvisioningTarget{Id: item.ID, ApplicationId: item.ApplicationID, EndpointUrl: item.EndpointURL, SigningAlgorithm: item.SigningAlgorithm, SecretFingerprint: item.SecretFingerprint, ActiveKeyVersion: item.ActiveKeyVersion, PreviousKeyVersion: valueOrZero(item.PreviousKeyVersion), PreviousValidUntil: optionalTimestamp(item.PreviousValidUntil), DeliveryStatus: item.DeliveryStatus, LastSuccessAt: optionalTimestamp(item.LastSuccessAt), LastFailureAt: optionalTimestamp(item.LastFailureAt), LastErrorCode: item.LastErrorCode, ConfigVersion: item.ConfigVersion, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)}
}

func onboardingChecksProto(items []portaldomain.OnboardingCheck) []*forgev1.PortalApplicationOnboardingCheck {
	out := make([]*forgev1.PortalApplicationOnboardingCheck, 0, len(items))
	for _, item := range items {
		out = append(out, &forgev1.PortalApplicationOnboardingCheck{Id: item.ID, ApplicationId: item.ApplicationID, ConfigVersion: item.ConfigVersion, CheckType: item.CheckType, Result: item.Result, ErrorCode: item.ErrorCode, EvidenceJson: item.EvidenceJSON, RequestId: item.RequestID, VerifiedBy: item.VerifiedBy, OccurredAt: timestamp(item.OccurredAt)})
	}
	return out
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
