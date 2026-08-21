package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	appcrypto "github.com/sevoniva-labs/velora/server/internal/platform/security/crypto"
	"github.com/sevoniva-labs/velora/server/internal/platform/security/mfa"
	"github.com/sevoniva-labs/velora/server/internal/platform/security/password"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrLocked = errors.New("account locked")
var ErrDisabled = errors.New("account disabled")
var ErrInvalidRole = errors.New("invalid role")
var ErrGrantCeiling = errors.New("grant ceiling exceeded")
var ErrInvalidLoginName = errors.New("invalid login name")
var ErrPasswordPolicy = errors.New("password policy violation")
var ErrPasswordReused = errors.New("password was used recently")
var ErrInvalidSecurityPolicy = errors.New("invalid security policy")
var ErrLastSystemAdmin = repository.ErrLastSystemAdmin
var ErrPasswordStateChanged = repository.ErrPasswordStateChanged
var ErrInteractiveSessionRequired = errors.New("interactive user session required")
var ErrInvalidDepartment = errors.New("invalid department")
var ErrInvalidPosition = errors.New("invalid position")
var ErrInvalidUserGroup = errors.New("invalid user group")
var ErrInvalidUserAssignment = errors.New("invalid user assignment")
var ErrInvalidMenu = errors.New("invalid menu")
var ErrInvalidDataScope = errors.New("invalid data scope")
var ErrMFARequired = errors.New("multi-factor authentication required")
var ErrInvalidMFA = errors.New("invalid multi-factor authentication code")
var ErrMFAAlreadyEnabled = repository.ErrMFAAlreadyEnabled
var ErrMFANotPending = errors.New("multi-factor authentication enrollment is not pending")
var ErrMFAUnavailable = errors.New("multi-factor authentication encryption is unavailable")
var ErrRoleConflict = errors.New("role combination violates segregation of duties")
var ErrStepUpRequired = errors.New("recent multi-factor authentication required")

type resolvedPolicy struct {
	passwordPolicy password.Policy
	sessionTTL     time.Duration
	maxFailures    int
	lockDuration   time.Duration
	maxAge         time.Duration
	history        int
	maxConcurrent  int
}

const (
	minimumPasswordLength     = 12
	minimumPasswordHistory    = 5
	maximumPasswordAgeDays    = 90
	maximumLoginFailures      = 5
	minimumLoginLockDuration  = 15 * time.Minute
	maximumSessionTTL         = 12 * time.Hour
	maximumConcurrentSessions = 5
	recentMFAWindow           = 10 * time.Minute
)

type Options struct {
	MinLength     int
	RequireUpper  bool
	RequireLower  bool
	RequireDigit  bool
	RequireSymbol bool
	History       int
	MaxAgeDays    int
	SessionTTL    time.Duration
	MaxFailures   int
	LockDuration  time.Duration
	Crypto        appcrypto.Provider
}

type Service struct {
	repo         *repository.IdentityRepo
	hasher       password.Hasher
	policy       password.Policy
	sessionTTL   time.Duration
	maxFailures  int
	lockDuration time.Duration
	history      int
	maxAge       time.Duration
	crypt        appcrypto.Provider
	totp         mfa.TOTP
}

func NewService(repo *repository.IdentityRepo, opt Options) *Service {
	return &Service{
		repo: repo, hasher: password.DefaultHasher(),
		policy:     password.Policy{MinLength: opt.MinLength, RequireUpper: opt.RequireUpper, RequireLower: opt.RequireLower, RequireDigit: opt.RequireDigit, RequireSymbol: opt.RequireSymbol},
		sessionTTL: opt.SessionTTL, maxFailures: opt.MaxFailures, lockDuration: opt.LockDuration, history: opt.History,
		maxAge: time.Duration(opt.MaxAgeDays) * 24 * time.Hour,
		crypt:  opt.Crypto,
	}
}

var basePermissions = []struct{ Key, Name string }{
	{"system.user.read", "查看用户"}, {"system.user.create", "创建用户"}, {"system.user.update", "修改用户"}, {"system.user.role.manage", "分配用户角色"},
	{"system.user.assignment.read", "查看用户任职"}, {"system.user.assignment.manage", "管理用户任职"},
	{"system.role.read", "查看角色权限"}, {"system.role.manage", "管理角色权限"},
	{"system.organization.read", "查看组织信息"}, {"system.organization.manage", "管理组织信息"},
	{"system.department.read", "查看部门"}, {"system.department.manage", "管理部门"},
	{"system.position.read", "查看岗位"}, {"system.position.manage", "管理岗位"},
	{"system.user_group.read", "查看用户组"}, {"system.user_group.manage", "管理用户组"},
	{"system.menu.read", "查看平台菜单"}, {"system.menu.manage", "管理平台菜单"},
	{"system.data_policy.read", "查看数据策略"}, {"system.data_policy.manage", "管理数据策略"}, {"system.data.export", "授权数据导出"}, {"system.data.retention.read", "查看数据保留证明"}, {"system.data.retention.manage", "记录数据删除证明"},
	{"system.identity_mapping.read", "查看外部身份绑定"}, {"system.identity_mapping.manage", "管理外部身份绑定"},
	{"system.access_review.read", "查看访问复核"}, {"system.access_review.manage", "管理访问复核"},
	{"system.session.read", "查看在线会话"}, {"system.session.revoke", "强制下线会话"},
	{"system.audit.read", "查看审计日志"}, {"system.audit.export", "导出审计日志"}, {"system.audit.verify", "校验审计完整性"},
	{"system.temporary_grant.read", "查看临时授权"}, {"system.temporary_grant.manage", "管理临时授权"},
	{"system.config.read", "查看系统配置"}, {"system.config.manage", "管理配置变更"}, {"system.security.manage", "管理安全配置"},
	{"portal.application.read", "查看门户应用"}, {"portal.application.manage", "管理门户应用"}, {"portal.application.publish", "发布门户应用"},
	{"iam.integration.read", "查看身份接入"}, {"iam.integration.manage", "管理身份接入"}, {"iam.integration.verify", "验证身份接入"}, {"iam.console.open", "打开身份管理控制台"},
	{"approval.request.create", "发起审批"}, {"approval.request.read", "查看审批"}, {"approval.task.decide", "处理审批"}, {"approval.task.transfer", "转办审批"}, {"approval.request.withdraw", "撤回审批"},
}

func (s *Service) Bootstrap(ctx context.Context, orgKey, orgName, admin, passwordRaw string) error {
	orgID, err := s.repo.EnsureOrganization(ctx, orgKey, orgName)
	if err != nil {
		return err
	}
	for _, r := range []struct{ k, n, scope string }{
		{"system_admin", "系统管理员", domain.DataScopeOrganization},
		{"security_admin", "安全管理员", domain.DataScopeOrganization},
		{"auditor", "审计员", domain.DataScopeOrganization},
		{"application_admin", "应用管理员", domain.DataScopeOrganization},
		{"iam_admin", "身份管理员", domain.DataScopeOrganization},
		{"user", "普通用户", domain.DataScopeSelf},
	} {
		if _, err = s.repo.EnsureRole(ctx, orgID, r.k, r.n, r.scope); err != nil {
			return err
		}
	}
	for _, rule := range []struct{ a, b, reason string }{
		{"system_admin", "security_admin", "系统管理与安全管理必须职责分离"},
		{"system_admin", "auditor", "系统管理与审计监督必须职责分离"},
		{"security_admin", "auditor", "安全管理与审计监督必须职责分离"},
	} {
		if err = s.repo.EnsureRoleConflict(ctx, orgID, rule.a, rule.b, rule.reason); err != nil {
			return err
		}
	}
	for _, p := range basePermissions {
		if _, err = s.repo.EnsurePermission(ctx, p.Key, p.Name); err != nil {
			return err
		}
	}
	// Keep system_admin as implicit superuser in code; seed explicit grants for
	// other built-in roles to make the model extensible without hard-coding
	// every endpoint to a role name.
	for _, k := range []string{"system.user.read", "system.user.assignment.read", "system.user.assignment.manage", "system.role.read", "system.organization.read", "system.organization.manage", "system.department.read", "system.department.manage", "system.position.read", "system.position.manage", "system.user_group.read", "system.user_group.manage", "system.menu.read", "system.menu.manage", "system.identity_mapping.read", "system.identity_mapping.manage", "system.access_review.read", "system.access_review.manage", "system.session.read", "system.session.revoke", "system.temporary_grant.read", "system.temporary_grant.manage", "system.config.read", "system.config.manage", "system.security.manage", "system.data_policy.read", "system.data_policy.manage", "system.data.export", "system.data.retention.read", "system.data.retention.manage", "portal.application.read", "portal.application.manage", "portal.application.publish", "iam.integration.read", "iam.integration.manage", "iam.integration.verify", "iam.console.open"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "security_admin", k); err != nil {
			return err
		}
	}
	for _, k := range []string{"portal.application.read", "portal.application.manage", "portal.application.publish"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "application_admin", k); err != nil {
			return err
		}
	}
	for _, k := range []string{"iam.integration.read", "iam.integration.manage", "iam.integration.verify", "iam.console.open"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "iam_admin", k); err != nil {
			return err
		}
	}
	for _, k := range []string{"system.audit.read", "system.audit.export", "system.audit.verify", "system.temporary_grant.read"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "auditor", k); err != nil {
			return err
		}
	}
	if err = s.repo.GrantPermissionToRole(ctx, orgID, "auditor", "system.data_policy.read"); err != nil {
		return err
	}
	for _, k := range []string{"approval.request.create", "approval.request.read", "approval.task.decide", "approval.task.transfer", "approval.request.withdraw"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "user", k); err != nil {
			return err
		}
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "security_admin", k); err != nil {
			return err
		}
	}
	if err = s.repo.GrantPermissionToRole(ctx, orgID, "user", "portal.application.read"); err != nil {
		return err
	}
	if err = s.repo.GrantPermissionToRole(ctx, orgID, "auditor", "approval.request.read"); err != nil {
		return err
	}
	for _, menu := range builtinMenus(orgID) {
		if err = s.repo.EnsureMenu(ctx, menu); err != nil {
			return err
		}
	}
	if admin == "" {
		return s.finalizeBootstrapDefaults(ctx, orgID)
	}
	if _, err = s.repo.UserByLogin(ctx, orgID, admin); err == nil {
		return s.finalizeBootstrapDefaults(ctx, orgID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err = s.policy.Validate(passwordRaw); err != nil {
		return fmt.Errorf("bootstrap password: %w", err)
	}
	h, err := s.hasher.Hash(passwordRaw)
	if err != nil {
		return err
	}
	u, err := s.repo.CreateUser(ctx, orgID, admin, "Administrator", h, true)
	if err != nil {
		// Multiple replicas can bootstrap concurrently. If another replica won
		// the unique (organization_id, login_name) race, re-read the account and
		// finish the idempotent role grant instead of failing this replica.
		existing, readErr := s.repo.UserByLogin(ctx, orgID, admin)
		if readErr != nil {
			return err
		}
		return s.repo.GrantRole(ctx, existing.User.ID, "system_admin")
	}
	if err = s.repo.GrantRole(ctx, u.ID, "system_admin"); err != nil {
		return err
	}
	return s.finalizeBootstrapDefaults(ctx, orgID)
}

func (s *Service) enforceOrganizationActive(ctx context.Context, orgID string) error {
	org, err := s.repo.OrganizationByID(ctx, orgID)
	if err != nil {
		return err
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return ErrDisabled
	}
	return nil
}

func (s *Service) defaultPolicy() resolvedPolicy {
	return resolvedPolicy{
		passwordPolicy: s.policy,
		sessionTTL:     s.sessionTTL,
		maxFailures:    s.maxFailures,
		lockDuration:   s.lockDuration,
		maxAge:         s.maxAge,
		history:        s.history,
		maxConcurrent:  maximumConcurrentSessions,
	}
}

func (s *Service) resolveSecurityPolicy(ctx context.Context, orgID string) (resolvedPolicy, error) {
	out := s.defaultPolicy()
	if orgID == "" {
		return out, validateResolvedPolicy(out)
	}
	settings, err := s.repo.SecuritySettings(ctx, orgID)
	if err != nil {
		return resolvedPolicy{}, err
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordMinLength]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 1 {
			return resolvedPolicy{}, fmt.Errorf("%w: password_min_length", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.MinLength = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireUpper]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_upper", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireUpper = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireLower]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_lower", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireLower = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireDigit]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_digit", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireDigit = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireSymbol]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_symbol", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireSymbol = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordHistory]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: password_history", ErrInvalidSecurityPolicy)
		}
		out.history = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordMaxAgeDays]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: password_max_age_days", ErrInvalidSecurityPolicy)
		}
		out.maxAge = time.Duration(parsed) * 24 * time.Hour
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingLoginMaxFailures]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: login_max_failures", ErrInvalidSecurityPolicy)
		}
		out.maxFailures = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingLoginLockDurationSec]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: login_lock_duration_seconds", ErrInvalidSecurityPolicy)
		}
		out.lockDuration = time.Duration(parsed) * time.Second
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingSessionTTLSeconds]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed <= 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: session_ttl_seconds", ErrInvalidSecurityPolicy)
		}
		out.sessionTTL = time.Duration(parsed) * time.Second
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingMaxConcurrentSessions]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: max_active_sessions", ErrInvalidSecurityPolicy)
		}
		out.maxConcurrent = parsed
	}
	if err := validateResolvedPolicy(out); err != nil {
		return resolvedPolicy{}, err
	}
	return out, nil
}

func (s *Service) finalizeBootstrapDefaults(ctx context.Context, orgID string) error {
	policy := s.defaultPolicy()
	existing, err := s.repo.SecuritySettings(ctx, orgID)
	if err != nil {
		return err
	}
	changes := map[string]string{
		domain.SecuritySettingPasswordMinLength:     strconv.Itoa(policy.passwordPolicy.MinLength),
		domain.SecuritySettingPasswordRequireUpper:  strconv.FormatBool(policy.passwordPolicy.RequireUpper),
		domain.SecuritySettingPasswordRequireLower:  strconv.FormatBool(policy.passwordPolicy.RequireLower),
		domain.SecuritySettingPasswordRequireDigit:  strconv.FormatBool(policy.passwordPolicy.RequireDigit),
		domain.SecuritySettingPasswordRequireSymbol: strconv.FormatBool(policy.passwordPolicy.RequireSymbol),
		domain.SecuritySettingPasswordHistory:       strconv.Itoa(policy.history),
		domain.SecuritySettingPasswordMaxAgeDays:    strconv.Itoa(int(policy.maxAge.Hours()) / 24),
		domain.SecuritySettingLoginMaxFailures:      strconv.Itoa(policy.maxFailures),
		domain.SecuritySettingLoginLockDurationSec:  strconv.FormatInt(int64(policy.lockDuration.Seconds()), 10),
		domain.SecuritySettingSessionTTLSeconds:     strconv.FormatInt(int64(policy.sessionTTL.Seconds()), 10),
		domain.SecuritySettingMaxConcurrentSessions: strconv.Itoa(policy.maxConcurrent),
	}
	for key := range changes {
		if _, ok := existing[key]; ok {
			delete(changes, key)
		}
	}
	if len(changes) == 0 {
		return nil
	}
	return s.repo.SetSecuritySettings(ctx, orgID, "system", changes)
}

func (s *Service) SecurityPolicy(ctx context.Context, orgID string) (domain.SecurityPolicy, error) {
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	return domain.SecurityPolicy{
		PasswordMinLength:        policy.passwordPolicy.MinLength,
		PasswordRequireUpper:     policy.passwordPolicy.RequireUpper,
		PasswordRequireLower:     policy.passwordPolicy.RequireLower,
		PasswordRequireDigit:     policy.passwordPolicy.RequireDigit,
		PasswordRequireSymbol:    policy.passwordPolicy.RequireSymbol,
		PasswordHistory:          policy.history,
		PasswordMaxAgeDays:       int(policy.maxAge.Hours()) / 24,
		LoginMaxFailures:         policy.maxFailures,
		LoginLockDurationSeconds: int64(policy.lockDuration.Seconds()),
		SessionTTLSeconds:        int64(policy.sessionTTL.Seconds()),
		MaxConcurrentSessions:    policy.maxConcurrent,
	}, nil
}

func (s *Service) UpdateSecurityPolicy(ctx context.Context, orgID, updatedBy string, policy domain.SecurityPolicy) (domain.SecurityPolicy, error) {
	if err := validateSecurityPolicy(policy); err != nil {
		return domain.SecurityPolicy{}, err
	}
	payload := map[string]string{
		domain.SecuritySettingPasswordMinLength:     strconv.Itoa(policy.PasswordMinLength),
		domain.SecuritySettingPasswordRequireUpper:  strconv.FormatBool(policy.PasswordRequireUpper),
		domain.SecuritySettingPasswordRequireLower:  strconv.FormatBool(policy.PasswordRequireLower),
		domain.SecuritySettingPasswordRequireDigit:  strconv.FormatBool(policy.PasswordRequireDigit),
		domain.SecuritySettingPasswordRequireSymbol: strconv.FormatBool(policy.PasswordRequireSymbol),
		domain.SecuritySettingPasswordHistory:       strconv.Itoa(policy.PasswordHistory),
		domain.SecuritySettingPasswordMaxAgeDays:    strconv.Itoa(policy.PasswordMaxAgeDays),
		domain.SecuritySettingLoginMaxFailures:      strconv.Itoa(policy.LoginMaxFailures),
		domain.SecuritySettingLoginLockDurationSec:  strconv.FormatInt(policy.LoginLockDurationSeconds, 10),
		domain.SecuritySettingSessionTTLSeconds:     strconv.FormatInt(policy.SessionTTLSeconds, 10),
		domain.SecuritySettingMaxConcurrentSessions: strconv.Itoa(policy.MaxConcurrentSessions),
	}
	if err := s.repo.SetSecuritySettings(ctx, orgID, updatedBy, payload); err != nil {
		return domain.SecurityPolicy{}, err
	}
	return s.SecurityPolicy(ctx, orgID)
}

func validateResolvedPolicy(policy resolvedPolicy) error {
	return validateSecurityPolicy(domain.SecurityPolicy{
		PasswordMinLength:        policy.passwordPolicy.MinLength,
		PasswordRequireUpper:     policy.passwordPolicy.RequireUpper,
		PasswordRequireLower:     policy.passwordPolicy.RequireLower,
		PasswordRequireDigit:     policy.passwordPolicy.RequireDigit,
		PasswordRequireSymbol:    policy.passwordPolicy.RequireSymbol,
		PasswordHistory:          policy.history,
		PasswordMaxAgeDays:       int(policy.maxAge.Hours()) / 24,
		LoginMaxFailures:         policy.maxFailures,
		LoginLockDurationSeconds: int64(policy.lockDuration.Seconds()),
		SessionTTLSeconds:        int64(policy.sessionTTL.Seconds()),
		MaxConcurrentSessions:    policy.maxConcurrent,
	})
}

func validateSecurityPolicy(policy domain.SecurityPolicy) error {
	switch {
	case policy.PasswordMinLength < minimumPasswordLength:
		return fmt.Errorf("%w: password_min_length must be at least %d", ErrInvalidSecurityPolicy, minimumPasswordLength)
	case !policy.PasswordRequireUpper || !policy.PasswordRequireLower || !policy.PasswordRequireDigit || !policy.PasswordRequireSymbol:
		return fmt.Errorf("%w: password character-class requirements cannot be disabled", ErrInvalidSecurityPolicy)
	case policy.PasswordHistory < minimumPasswordHistory:
		return fmt.Errorf("%w: password_history must be at least %d", ErrInvalidSecurityPolicy, minimumPasswordHistory)
	case policy.PasswordMaxAgeDays < 1 || policy.PasswordMaxAgeDays > maximumPasswordAgeDays:
		return fmt.Errorf("%w: password_max_age_days must be between 1 and %d", ErrInvalidSecurityPolicy, maximumPasswordAgeDays)
	case policy.LoginMaxFailures < 1 || policy.LoginMaxFailures > maximumLoginFailures:
		return fmt.Errorf("%w: login_max_failures must be between 1 and %d", ErrInvalidSecurityPolicy, maximumLoginFailures)
	case time.Duration(policy.LoginLockDurationSeconds)*time.Second < minimumLoginLockDuration:
		return fmt.Errorf("%w: login_lock_duration_seconds must be at least %d", ErrInvalidSecurityPolicy, int64(minimumLoginLockDuration.Seconds()))
	case policy.SessionTTLSeconds < 1 || time.Duration(policy.SessionTTLSeconds)*time.Second > maximumSessionTTL:
		return fmt.Errorf("%w: session_ttl_seconds must be between 1 and %d", ErrInvalidSecurityPolicy, int64(maximumSessionTTL.Seconds()))
	case policy.MaxConcurrentSessions < 1 || policy.MaxConcurrentSessions > maximumConcurrentSessions:
		return fmt.Errorf("%w: max_active_sessions must be between 1 and %d", ErrInvalidSecurityPolicy, maximumConcurrentSessions)
	default:
		return nil
	}
}

func (s *Service) UpdateOrganization(ctx context.Context, orgID string, req domain.Organization) (domain.Organization, error) {
	if req.Name = strings.TrimSpace(req.Name); req.Name == "" {
		return domain.Organization{}, fmt.Errorf("invalid organization name")
	}
	if req.Status == "" {
		existing, err := s.repo.OrganizationByID(ctx, orgID)
		if err != nil {
			return domain.Organization{}, err
		}
		req.Status = existing.Status
	}
	req.Status = strings.ToUpper(req.Status)
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return domain.Organization{}, fmt.Errorf("invalid organization status")
	}
	if req.MaxUsers < 0 || req.MaxSessions < 0 {
		return domain.Organization{}, fmt.Errorf("invalid organization value")
	}
	return s.repo.UpdateOrganization(ctx, orgID, req)
}

func (s *Service) enforceMaxConcurrentSessions(ctx context.Context, userID string, maxSessions int) error {
	if maxSessions <= 0 {
		return nil
	}
	ids, err := s.repo.ListUserSessionIDs(ctx, userID)
	if err != nil {
		return err
	}
	excess := len(ids) - maxSessions
	if excess <= 0 {
		return nil
	}
	for i := 0; i < excess; i++ {
		if err := s.repo.DeleteSessionByID(ctx, ids[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) passwordExpiredAt(t time.Time, maxAge time.Duration) bool {
	return maxAge > 0 && !t.IsZero() && time.Since(t) > maxAge
}

func (s *Service) Login(ctx context.Context, orgID, login, raw, ip, ua string) (domain.Principal, string, string, time.Time, error) {
	return s.LoginWithMFA(ctx, orgID, login, raw, "", "", ip, ua)
}

func (s *Service) LoginWithMFA(ctx context.Context, orgID, login, raw, mfaCode, recoveryCode, ip, ua string) (domain.Principal, string, string, time.Time, error) {
	row, err := s.repo.UserByLogin(ctx, orgID, strings.TrimSpace(login))
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	org, err := s.repo.OrganizationByID(ctx, row.User.OrganizationID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrDisabled
	}
	policy, err := s.resolveSecurityPolicy(ctx, row.User.OrganizationID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if row.User.Status != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrDisabled
	}
	if row.User.LockedUntil != nil && time.Now().Before(*row.User.LockedUntil) {
		return domain.Principal{}, "", "", *row.User.LockedUntil, ErrLocked
	}
	if !s.hasher.Verify(raw, row.PasswordHash) {
		if err := s.repo.RecordLoginFailure(ctx, row.User.ID, policy.maxFailures, policy.lockDuration); err != nil {
			return domain.Principal{}, "", "", time.Time{}, err
		}
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	mfaVerified, err := s.verifyLoginMFA(ctx, row.User.ID, mfaCode, recoveryCode)
	if err != nil {
		if errors.Is(err, ErrInvalidMFA) {
			if recordErr := s.repo.RecordLoginFailure(ctx, row.User.ID, policy.maxFailures, policy.lockDuration); recordErr != nil {
				return domain.Principal{}, "", "", time.Time{}, recordErr
			}
		}
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if err := s.repo.ResetLoginFailure(ctx, row.User.ID); err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(policy.sessionTTL)
	authenticationLevel := "PASSWORD"
	var mfaVerifiedAt *time.Time
	if mfaVerified {
		authenticationLevel = "MFA"
		verifiedAt := time.Now().UTC()
		mfaVerifiedAt = &verifiedAt
	}
	sessionID, err := s.repo.CreateSession(ctx, row.User.ID, hashToken(token), expires, ip, ua, authenticationLevel, mfaVerifiedAt)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if err := s.enforceMaxConcurrentSessions(ctx, row.User.ID, policy.maxConcurrent); err != nil {
		_ = s.repo.DeleteSessionByID(ctx, sessionID)
		return domain.Principal{}, "", "", time.Time{}, err
	}
	mustChange := row.User.MustChangePassword || s.passwordExpiredAt(row.User.PasswordChangedAt, policy.maxAge)
	p := domain.Principal{Type: "USER", UserID: row.User.ID, OrganizationID: row.User.OrganizationID, LoginName: row.User.LoginName, DisplayName: row.User.DisplayName, Roles: row.User.Roles, Permissions: row.User.Permissions, MustChangePassword: mustChange, SessionID: sessionID, PasswordChangedAt: row.User.PasswordChangedAt, AuthenticationLevel: authenticationLevel, MFAVerifiedAt: mfaVerifiedAt}
	return p, token, csrf, expires, nil
}

// LoginFederated completes a provider-authenticated login. The provider
// adapter must authenticate the credential first; this method only accepts an
// explicit, approved subject mapping and never provisions or matches by
// email/login name.
func (s *Service) LoginFederated(ctx context.Context, orgID, provider, subject, ip, ua string) (domain.Principal, string, string, time.Time, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	if !federatedProviderPattern.MatchString(provider) || subject == "" || len(subject) > 512 {
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	link, err := s.repo.FederatedIdentityByProviderSubject(ctx, orgID, provider, subject)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	user, err := s.repo.UserByID(ctx, link.UserID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if user.OrganizationID != orgID || strings.ToUpper(user.Status) != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	org, err := s.repo.OrganizationByID(ctx, orgID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrDisabled
	}
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(policy.sessionTTL)
	sessionID, err := s.repo.CreateSession(ctx, user.ID, hashToken(token), expires, ip, ua, "FEDERATED", nil)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if err := s.enforceMaxConcurrentSessions(ctx, user.ID, policy.maxConcurrent); err != nil {
		_ = s.repo.DeleteSessionByID(ctx, sessionID)
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if err := s.repo.ResetLoginFailure(ctx, user.ID); err != nil {
		_ = s.repo.DeleteSessionByID(ctx, sessionID)
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if err := s.repo.TouchFederatedIdentityAuthentication(ctx, orgID, link.ID, time.Now().UTC()); err != nil {
		_ = s.repo.DeleteSessionByID(ctx, sessionID)
		return domain.Principal{}, "", "", time.Time{}, err
	}
	mustChange := user.MustChangePassword || s.passwordExpiredAt(user.PasswordChangedAt, policy.maxAge)
	p := domain.Principal{Type: "USER", UserID: user.ID, OrganizationID: user.OrganizationID, LoginName: user.LoginName, DisplayName: user.DisplayName, Roles: user.Roles, Permissions: user.Permissions, MustChangePassword: mustChange, SessionID: sessionID, PasswordChangedAt: user.PasswordChangedAt, AuthenticationLevel: "FEDERATED"}
	return p, token, csrf, expires, nil
}

func (s *Service) verifyLoginMFA(ctx context.Context, userID, code, recoveryCode string) (bool, error) {
	factor, err := s.repo.ActiveMFAFactor(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(code) == "" && strings.TrimSpace(recoveryCode) == "" {
		return false, ErrMFARequired
	}
	if err := s.verifyMFAFactor(ctx, factor, code, recoveryCode); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) StepUpAuthentication(ctx context.Context, actor domain.Principal, currentPassword, code, recoveryCode string) (time.Time, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return time.Time{}, err
	}
	hash, err := s.repo.PasswordHashByID(ctx, actor.UserID)
	if err != nil {
		return time.Time{}, err
	}
	if !s.hasher.Verify(currentPassword, hash) {
		return time.Time{}, ErrInvalidCredentials
	}
	factor, err := s.repo.ActiveMFAFactor(ctx, actor.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrMFARequired
	}
	if err != nil {
		return time.Time{}, err
	}
	if err := s.verifyMFAFactor(ctx, factor, code, recoveryCode); err != nil {
		return time.Time{}, err
	}
	verifiedAt := time.Now().UTC()
	if err := s.repo.MarkSessionMFAVerified(ctx, actor.UserID, actor.SessionID, verifiedAt); err != nil {
		return time.Time{}, err
	}
	return verifiedAt, nil
}

func RequireRecentMFA(actor domain.Principal) error {
	if actor.MFAVerifiedAt == nil {
		return ErrStepUpRequired
	}
	age := time.Since(*actor.MFAVerifiedAt)
	if age < 0 || age > recentMFAWindow {
		return ErrStepUpRequired
	}
	return nil
}

func (s *Service) verifyMFAFactor(ctx context.Context, factor domain.MFAFactor, code, recoveryCode string) error {
	if s.crypt == nil {
		return ErrMFAUnavailable
	}
	if code = strings.TrimSpace(code); code != "" {
		secret, err := s.crypt.Decrypt(factor.SecretCiphertext, []byte("mfa:totp:"+factor.UserID))
		if err != nil {
			return err
		}
		if s.totp.Validate(code, string(secret)) {
			return nil
		}
	}
	if recoveryCode = strings.TrimSpace(recoveryCode); recoveryCode != "" {
		consumed, err := s.repo.ConsumeMFARecoveryCode(ctx, factor.UserID, s.recoveryCodeHash(factor.UserID, recoveryCode))
		if err != nil {
			return err
		}
		if consumed {
			return nil
		}
	}
	return ErrInvalidMFA
}

func (s *Service) MFAEnabled(ctx context.Context, actor domain.Principal) (bool, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return false, err
	}
	return s.repo.MFAEnabled(ctx, actor.UserID)
}

func (s *Service) BeginMFAEnrollment(ctx context.Context, actor domain.Principal, currentPassword string) (domain.MFAEnrollment, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return domain.MFAEnrollment{}, err
	}
	if s.crypt == nil {
		return domain.MFAEnrollment{}, ErrMFAUnavailable
	}
	hash, err := s.repo.PasswordHashByID(ctx, actor.UserID)
	if err != nil {
		return domain.MFAEnrollment{}, err
	}
	if !s.hasher.Verify(currentPassword, hash) {
		return domain.MFAEnrollment{}, ErrInvalidCredentials
	}
	secret, url, err := s.totp.Generate("Velora", actor.LoginName)
	if err != nil {
		return domain.MFAEnrollment{}, err
	}
	ciphertext, err := s.crypt.Encrypt([]byte(secret), []byte("mfa:totp:"+actor.UserID))
	if err != nil {
		return domain.MFAEnrollment{}, err
	}
	if err := s.repo.SavePendingMFAFactor(ctx, actor.UserID, ciphertext, s.crypt.KeyVersion(), time.Now().UTC().Add(10*time.Minute)); err != nil {
		return domain.MFAEnrollment{}, err
	}
	return domain.MFAEnrollment{Secret: secret, URL: url}, nil
}

func (s *Service) ConfirmMFAEnrollment(ctx context.Context, actor domain.Principal, code string) ([]string, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return nil, err
	}
	factor, err := s.repo.PendingMFAFactor(ctx, actor.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMFANotPending
	}
	if err != nil {
		return nil, err
	}
	if err := s.verifyMFAFactor(ctx, factor, code, ""); err != nil {
		return nil, err
	}
	codes := make([]string, 10)
	hashes := make([]string, len(codes))
	for index := range codes {
		codes[index], err = randomToken(12)
		if err != nil {
			return nil, err
		}
		hashes[index] = s.recoveryCodeHash(actor.UserID, codes[index])
	}
	if err := s.repo.ActivateMFAFactor(ctx, actor.UserID, hashes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Service) DisableMFA(ctx context.Context, actor domain.Principal, currentPassword, code, recoveryCode string) error {
	if err := requireInteractivePrincipal(actor); err != nil {
		return err
	}
	hash, err := s.repo.PasswordHashByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(currentPassword, hash) {
		return ErrInvalidCredentials
	}
	factor, err := s.repo.ActiveMFAFactor(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if err := s.verifyMFAFactor(ctx, factor, code, recoveryCode); err != nil {
		return err
	}
	return s.repo.DeleteMFAAndOtherSessions(ctx, actor.UserID, actor.SessionID)
}

func (s *Service) recoveryCodeHash(userID, code string) string {
	return hex.EncodeToString(s.crypt.Hash([]byte("mfa-recovery:" + userID + ":" + strings.TrimSpace(code))))
}
func (s *Service) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if token == "" {
		return domain.Principal{}, sql.ErrNoRows
	}
	p, err := s.repo.PrincipalBySessionHash(ctx, hashToken(token))
	if err == nil {
		org, e := s.repo.OrganizationByID(ctx, p.OrganizationID)
		if e != nil {
			return domain.Principal{}, e
		}
		if strings.ToUpper(org.Status) != "ACTIVE" {
			return domain.Principal{}, ErrDisabled
		}
		policy, policyErr := s.resolveSecurityPolicy(ctx, p.OrganizationID)
		if policyErr != nil {
			return domain.Principal{}, policyErr
		}
		if s.passwordExpiredAt(p.PasswordChangedAt, policy.maxAge) {
			p.MustChangePassword = true
		}
	}
	return p, err
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSessionByHash(ctx, hashToken(token))
}
func (s *Service) ListUsers(ctx context.Context, actor domain.Principal) ([]domain.User, error) {
	return s.repo.ListUsers(ctx, actor.OrganizationID, actor.UserID, actor.DataScope, 200)
}
func (s *Service) CreateUser(ctx context.Context, actor domain.Principal, orgID, login, display, raw string, roles []string) (domain.User, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.User{}, err
	}
	if _, err := s.repo.OrganizationByID(ctx, orgID); err != nil {
		return domain.User{}, err
	}
	if err := s.enforceOrganizationActive(ctx, orgID); err != nil {
		return domain.User{}, err
	}
	login = strings.TrimSpace(login)
	display = strings.TrimSpace(display)
	if login == "" || len(login) > 120 {
		return domain.User{}, ErrInvalidLoginName
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}
	allowed := map[string]struct{}{"system_admin": {}, "security_admin": {}, "auditor": {}, "user": {}}
	for _, r := range roles {
		if _, ok := allowed[r]; !ok {
			return domain.User{}, ErrInvalidRole
		}
	}
	if err := enforceRoleMutation(actor, nil, roles); err != nil {
		return domain.User{}, err
	}
	if err := s.validateRoleCombination(ctx, orgID, roles); err != nil {
		return domain.User{}, err
	}
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return domain.User{}, err
	}
	if err := policy.passwordPolicy.Validate(raw); err != nil {
		return domain.User{}, fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	h, err := s.hasher.Hash(raw)
	if err != nil {
		return domain.User{}, err
	}
	return s.repo.CreateUserWithRoles(ctx, orgID, login, display, h, true, roles)
}
func (s *Service) ChangePassword(ctx context.Context, actor domain.Principal, current, next string) error {
	if err := requireInteractivePrincipal(actor); err != nil {
		return err
	}
	user, err := s.repo.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	policy, err := s.resolveSecurityPolicy(ctx, user.OrganizationID)
	if err != nil {
		return err
	}
	if err := s.enforceOrganizationActive(ctx, user.OrganizationID); err != nil {
		return err
	}
	if err := policy.passwordPolicy.Validate(next); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	currentHash, err := s.repo.PasswordHashByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(current, currentHash) {
		return ErrInvalidCredentials
	}
	if s.hasher.Verify(next, currentHash) {
		return ErrPasswordReused
	}
	if policy.history > 0 {
		history, err := s.repo.PasswordHistory(ctx, actor.UserID, policy.history)
		if err != nil {
			return err
		}
		for _, old := range history {
			if s.hasher.Verify(next, old) {
				return ErrPasswordReused
			}
		}
	}
	nextHash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordAndRevokeOtherSessions(ctx, actor.UserID, actor.SessionID, currentHash, nextHash)
}
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func hashToken(v string) string { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }

func (s *Service) CreateAPIToken(ctx context.Context, actor domain.Principal, name string, scopes []string, ttl time.Duration) (domain.APIToken, string, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return domain.APIToken{}, "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return domain.APIToken{}, "", errors.New("invalid token name")
	}
	u, err := s.repo.UserByID(ctx, actor.UserID)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	policy, err := s.resolveSecurityPolicy(ctx, u.OrganizationID)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	if u.MustChangePassword || s.passwordExpiredAt(u.PasswordChangedAt, policy.maxAge) {
		return domain.APIToken{}, "", ErrPasswordPolicy
	}
	allowed := map[string]struct{}{}
	for _, p := range u.Permissions {
		allowed[p] = struct{}{}
	}
	if u.HasRole("system_admin") {
		allowed["*"] = struct{}{}
	}
	if len(scopes) == 0 {
		if u.HasRole("system_admin") {
			scopes = []string{"*"}
		} else {
			scopes = append([]string(nil), u.Permissions...)
		}
	}
	for _, scope := range scopes {
		if _, ok := allowed[scope]; !ok {
			return domain.APIToken{}, "", ErrInvalidRole
		}
	}
	if ttl <= 0 {
		ttl = 90 * 24 * time.Hour
	}
	if ttl > 365*24*time.Hour {
		return domain.APIToken{}, "", errors.New("api token ttl exceeds 365 days")
	}
	expires := time.Now().UTC().Add(ttl)
	random, err := randomToken(32)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	raw := "fg_" + random
	prefix := raw
	if len(prefix) > 15 {
		prefix = prefix[:15]
	}
	t, err := s.repo.CreateAPIToken(ctx, actor.UserID, name, prefix, hashToken(raw), scopes, &expires)
	return t, raw, err
}
func (s *Service) ListAPITokens(ctx context.Context, actor domain.Principal) ([]domain.APIToken, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return nil, err
	}
	return s.repo.ListAPITokens(ctx, actor.UserID)
}
func (s *Service) RevokeAPIToken(ctx context.Context, actor domain.Principal, tokenID string) error {
	if err := requireInteractivePrincipal(actor); err != nil {
		return err
	}
	return s.repo.RevokeAPIToken(ctx, actor.UserID, tokenID)
}

func requireInteractivePrincipal(actor domain.Principal) error {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" || actor.SessionID == "" {
		return ErrInteractiveSessionRequired
	}
	return nil
}
func (s *Service) AuthenticateAPIToken(ctx context.Context, raw string) (domain.Principal, error) {
	if !strings.HasPrefix(raw, "fg_") {
		return domain.Principal{}, ErrInvalidCredentials
	}
	p, err := s.repo.PrincipalByAPITokenHash(ctx, hashToken(raw))
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	org, err := s.repo.OrganizationByID(ctx, p.OrganizationID)
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return domain.Principal{}, ErrInvalidCredentials
	}
	policy, err := s.resolveSecurityPolicy(ctx, p.OrganizationID)
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if p.MustChangePassword || s.passwordExpiredAt(p.PasswordChangedAt, policy.maxAge) {
		return domain.Principal{}, ErrInvalidCredentials
	}
	return p, nil
}

func (s *Service) Organization(ctx context.Context, orgID string) (domain.Organization, error) {
	return s.repo.OrganizationByID(ctx, orgID)
}

func (s *Service) ListDepartments(ctx context.Context, orgID string) ([]domain.Department, error) {
	return s.repo.ListDepartments(ctx, orgID)
}

func (s *Service) CreateDepartment(ctx context.Context, actor domain.Principal, orgID string, req domain.Department) (domain.Department, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Department{}, err
	}
	req.OrganizationID = orgID
	clean, err := normalizeDepartment(req, true)
	if err != nil {
		return domain.Department{}, err
	}
	items, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return domain.Department{}, err
	}
	if err := validateDepartmentHierarchy(items, clean, true); err != nil {
		return domain.Department{}, err
	}
	return s.repo.CreateDepartment(ctx, clean)
}

func (s *Service) UpdateDepartment(ctx context.Context, actor domain.Principal, orgID, departmentID string, req domain.Department) (domain.Department, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Department{}, err
	}
	departmentID = strings.TrimSpace(departmentID)
	if departmentID == "" {
		return domain.Department{}, ErrInvalidDepartment
	}
	items, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return domain.Department{}, err
	}
	var current domain.Department
	found := false
	for _, item := range items {
		if item.ID == departmentID {
			current = item
			found = true
			break
		}
	}
	if !found {
		return domain.Department{}, sql.ErrNoRows
	}
	req.ID = current.ID
	req.OrganizationID = orgID
	req.Key = current.Key
	req.CreatedAt = current.CreatedAt
	clean, err := normalizeDepartment(req, false)
	if err != nil {
		return domain.Department{}, err
	}
	if err := validateDepartmentHierarchy(items, clean, false); err != nil {
		return domain.Department{}, err
	}
	if clean.Status == "DISABLED" {
		active, err := s.repo.CountActivePositions(ctx, orgID, clean.ID)
		if err != nil {
			return domain.Department{}, err
		}
		if active > 0 {
			return domain.Department{}, ErrInvalidDepartment
		}
		assigned, err := s.repo.CountCurrentAssignments(ctx, orgID, clean.ID, time.Now().UTC())
		if err != nil {
			return domain.Department{}, err
		}
		if assigned > 0 {
			return domain.Department{}, ErrInvalidDepartment
		}
	}
	return s.repo.UpdateDepartment(ctx, clean)
}

func normalizeDepartment(req domain.Department, creating bool) (domain.Department, error) {
	req.ID = strings.TrimSpace(req.ID)
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	req.ParentID = strings.TrimSpace(req.ParentID)
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if creating && req.Status == "" {
		req.Status = "ACTIVE"
	}
	if req.OrganizationID == "" || req.Name == "" || len(req.Name) > 200 || req.SortOrder < 0 || req.SortOrder > 1_000_000 {
		return domain.Department{}, ErrInvalidDepartment
	}
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return domain.Department{}, ErrInvalidDepartment
	}
	if creating && (req.Key == "" || len(req.Key) > 100 || !validDirectoryKey(req.Key)) {
		return domain.Department{}, ErrInvalidDepartment
	}
	return req, nil
}

func validDirectoryKey(value string) bool {
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateDepartmentHierarchy(items []domain.Department, candidate domain.Department, creating bool) error {
	byID := make(map[string]domain.Department, len(items)+1)
	for _, item := range items {
		byID[item.ID] = item
	}
	if !creating {
		if _, ok := byID[candidate.ID]; !ok {
			return sql.ErrNoRows
		}
		byID[candidate.ID] = candidate
	}
	if candidate.ParentID != "" {
		parent, ok := byID[candidate.ParentID]
		if !ok || parent.OrganizationID != candidate.OrganizationID || parent.ID == candidate.ID {
			return ErrInvalidDepartment
		}
		if candidate.Status == "ACTIVE" && parent.Status != "ACTIVE" {
			return ErrInvalidDepartment
		}
	}
	seen := map[string]struct{}{candidate.ID: {}}
	for parentID := candidate.ParentID; parentID != ""; {
		if _, exists := seen[parentID]; exists {
			return ErrInvalidDepartment
		}
		seen[parentID] = struct{}{}
		parent, ok := byID[parentID]
		if !ok {
			return ErrInvalidDepartment
		}
		parentID = parent.ParentID
	}
	if candidate.Status == "DISABLED" {
		for _, item := range byID {
			if item.ParentID == candidate.ID && item.Status == "ACTIVE" {
				return ErrInvalidDepartment
			}
		}
	}
	return nil
}

func (s *Service) ListPositions(ctx context.Context, orgID string) ([]domain.Position, error) {
	return s.repo.ListPositions(ctx, orgID)
}

func (s *Service) CreatePosition(ctx context.Context, actor domain.Principal, orgID string, req domain.Position) (domain.Position, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Position{}, err
	}
	req.OrganizationID = orgID
	clean, err := normalizePosition(req, true)
	if err != nil {
		return domain.Position{}, err
	}
	departments, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return domain.Position{}, err
	}
	if err := validatePositionDepartment(departments, clean); err != nil {
		return domain.Position{}, err
	}
	return s.repo.CreatePosition(ctx, clean)
}

func (s *Service) UpdatePosition(ctx context.Context, actor domain.Principal, orgID, positionID string, req domain.Position) (domain.Position, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Position{}, err
	}
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return domain.Position{}, ErrInvalidPosition
	}
	current, err := s.repo.PositionByID(ctx, orgID, positionID)
	if err != nil {
		return domain.Position{}, err
	}
	req.ID = current.ID
	req.OrganizationID = orgID
	req.Key = current.Key
	req.CreatedAt = current.CreatedAt
	clean, err := normalizePosition(req, false)
	if err != nil {
		return domain.Position{}, err
	}
	departments, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return domain.Position{}, err
	}
	if err := validatePositionDepartment(departments, clean); err != nil {
		return domain.Position{}, err
	}
	return s.repo.UpdatePosition(ctx, clean)
}

func normalizePosition(req domain.Position, creating bool) (domain.Position, error) {
	req.ID = strings.TrimSpace(req.ID)
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	req.DepartmentID = strings.TrimSpace(req.DepartmentID)
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if creating && req.Status == "" {
		req.Status = "ACTIVE"
	}
	if req.OrganizationID == "" || req.DepartmentID == "" || req.Name == "" || len(req.Name) > 200 || len(req.Description) > 500 || req.SortOrder < 0 || req.SortOrder > 1_000_000 {
		return domain.Position{}, ErrInvalidPosition
	}
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return domain.Position{}, ErrInvalidPosition
	}
	if creating && (req.Key == "" || len(req.Key) > 100 || !validDirectoryKey(req.Key)) {
		return domain.Position{}, ErrInvalidPosition
	}
	return req, nil
}

func validatePositionDepartment(departments []domain.Department, position domain.Position) error {
	for _, department := range departments {
		if department.ID != position.DepartmentID {
			continue
		}
		if department.OrganizationID != position.OrganizationID || (position.Status == "ACTIVE" && department.Status != "ACTIVE") {
			return ErrInvalidPosition
		}
		return nil
	}
	return ErrInvalidPosition
}

func (s *Service) ListUserGroups(ctx context.Context, orgID string) ([]domain.UserGroup, error) {
	return s.repo.ListUserGroups(ctx, orgID)
}

func (s *Service) CreateUserGroup(ctx context.Context, actor domain.Principal, orgID string, req domain.UserGroup) (domain.UserGroup, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.UserGroup{}, err
	}
	req.OrganizationID = orgID
	clean, err := normalizeUserGroup(req, true)
	if err != nil {
		return domain.UserGroup{}, err
	}
	return s.repo.CreateUserGroup(ctx, clean)
}

func (s *Service) UpdateUserGroup(ctx context.Context, actor domain.Principal, orgID, groupID string, req domain.UserGroup) (domain.UserGroup, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.UserGroup{}, err
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return domain.UserGroup{}, ErrInvalidUserGroup
	}
	current, err := s.repo.UserGroupByID(ctx, orgID, groupID)
	if err != nil {
		return domain.UserGroup{}, err
	}
	req.ID = current.ID
	req.OrganizationID = orgID
	req.Key = current.Key
	req.CreatedAt = current.CreatedAt
	clean, err := normalizeUserGroup(req, false)
	if err != nil {
		return domain.UserGroup{}, err
	}
	if current.Status != clean.Status {
		if err := enforceRoleMutation(actor, current.Roles, nil); err != nil {
			return domain.UserGroup{}, err
		}
		if clean.Status == "ACTIVE" {
			if err := s.validateGroupMembers(ctx, orgID, current.ID, current.Roles, current.MemberIDs); err != nil {
				return domain.UserGroup{}, err
			}
		}
	}
	return s.repo.UpdateUserGroup(ctx, clean)
}

func (s *Service) UpdateUserGroupMembers(ctx context.Context, actor domain.Principal, orgID, groupID string, memberIDs []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
	group, err := s.repo.UserGroupByID(ctx, orgID, strings.TrimSpace(groupID))
	if err != nil {
		return err
	}
	if err := enforceRoleMutation(actor, nil, group.Roles); err != nil {
		return err
	}
	clean, err := normalizeIDs(memberIDs, 10_000)
	if err != nil {
		return ErrInvalidUserGroup
	}
	if err := s.validateGroupMembers(ctx, orgID, groupID, group.Roles, clean); err != nil {
		return err
	}
	return s.repo.ReplaceUserGroupMembers(ctx, orgID, groupID, clean)
}

func (s *Service) UpdateUserGroupRoles(ctx context.Context, actor domain.Principal, orgID, groupID string, roleKeys []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
	group, err := s.repo.UserGroupByID(ctx, orgID, strings.TrimSpace(groupID))
	if err != nil {
		return err
	}
	clean, err := normalizeIDs(roleKeys, 200)
	if err != nil || contains(clean, "system_admin") {
		return ErrInvalidRole
	}
	roles, err := s.repo.ListRoles(ctx, orgID)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role.Key] = struct{}{}
	}
	for _, role := range clean {
		if _, ok := allowed[role]; !ok {
			return ErrInvalidRole
		}
	}
	if err := enforceRoleMutation(actor, group.Roles, clean); err != nil {
		return err
	}
	if err := s.validateGroupMembers(ctx, orgID, groupID, clean, group.MemberIDs); err != nil {
		return err
	}
	return s.repo.ReplaceUserGroupRoles(ctx, orgID, groupID, clean)
}

func normalizeUserGroup(req domain.UserGroup, creating bool) (domain.UserGroup, error) {
	req.ID = strings.TrimSpace(req.ID)
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if creating && req.Status == "" {
		req.Status = "ACTIVE"
	}
	if req.OrganizationID == "" || req.Name == "" || len(req.Name) > 200 || len(req.Description) > 500 {
		return domain.UserGroup{}, ErrInvalidUserGroup
	}
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return domain.UserGroup{}, ErrInvalidUserGroup
	}
	if creating && (req.Key == "" || len(req.Key) > 100 || !validDirectoryKey(req.Key)) {
		return domain.UserGroup{}, ErrInvalidUserGroup
	}
	return req, nil
}

func normalizeIDs(values []string, maximum int) ([]string, error) {
	if len(values) > maximum {
		return nil, errors.New("too many values")
	}
	seen := make(map[string]struct{}, len(values))
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 160 {
			return nil, errors.New("invalid value")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	return clean, nil
}

func (s *Service) ListUserAssignments(ctx context.Context, actor domain.Principal, userID string) ([]domain.UserAssignment, error) {
	userID = strings.TrimSpace(userID)
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.OrganizationID != actor.OrganizationID {
		return nil, sql.ErrNoRows
	}
	visible, err := s.userWithinDataScope(ctx, actor, userID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrGrantCeiling
	}
	return s.repo.ListUserAssignments(ctx, actor.OrganizationID, userID)
}

func (s *Service) ReplaceUserAssignments(ctx context.Context, actor domain.Principal, orgID, userID string, assignments []domain.UserAssignment) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrganizationID != orgID {
		return sql.ErrNoRows
	}
	visible, err := s.userWithinDataScope(ctx, actor, userID)
	if err != nil {
		return err
	}
	if !visible {
		return ErrGrantCeiling
	}
	clean, err := normalizeUserAssignments(orgID, userID, assignments, time.Now().UTC())
	if err != nil {
		return err
	}
	departments, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return err
	}
	positions, err := s.repo.ListPositions(ctx, orgID)
	if err != nil {
		return err
	}
	if err := validateAssignmentTargets(departments, positions, clean); err != nil {
		return err
	}
	return s.repo.ReplaceUserAssignments(ctx, orgID, userID, clean)
}

func (s *Service) userWithinDataScope(ctx context.Context, actor domain.Principal, userID string) (bool, error) {
	if actor.OrganizationID == "" || actor.UserID == "" {
		return false, nil
	}
	if actor.DataScope.OrganizationWide || actor.HasRole("system_admin") {
		return true, nil
	}
	if actor.DataScope.Self && actor.UserID == userID {
		return true, nil
	}
	departments, err := s.repo.CurrentUserDepartmentIDs(ctx, actor.OrganizationID, userID, time.Now().UTC())
	if err != nil {
		return false, err
	}
	for _, departmentID := range departments {
		if actor.DataScope.Allows(userID, departmentID, actor.UserID) {
			return true, nil
		}
	}
	return false, nil
}

func normalizeUserAssignments(orgID, userID string, assignments []domain.UserAssignment, now time.Time) ([]domain.UserAssignment, error) {
	if len(assignments) > 50 {
		return nil, ErrInvalidUserAssignment
	}
	clean := make([]domain.UserAssignment, 0, len(assignments))
	seen := make(map[string]struct{}, len(assignments))
	primaryCount := 0
	for _, assignment := range assignments {
		assignment.ID = ""
		assignment.OrganizationID = orgID
		assignment.UserID = userID
		assignment.DepartmentID = strings.TrimSpace(assignment.DepartmentID)
		assignment.PositionID = strings.TrimSpace(assignment.PositionID)
		if assignment.DepartmentID == "" {
			return nil, ErrInvalidUserAssignment
		}
		if assignment.ValidFrom.IsZero() {
			assignment.ValidFrom = now
		} else {
			assignment.ValidFrom = assignment.ValidFrom.UTC()
		}
		if assignment.ValidUntil != nil {
			until := assignment.ValidUntil.UTC()
			if !until.After(assignment.ValidFrom) {
				return nil, ErrInvalidUserAssignment
			}
			assignment.ValidUntil = &until
		}
		key := assignment.DepartmentID + "\x00" + assignment.PositionID
		if _, exists := seen[key]; exists {
			return nil, ErrInvalidUserAssignment
		}
		seen[key] = struct{}{}
		if assignment.Primary {
			primaryCount++
		}
		clean = append(clean, assignment)
	}
	if len(clean) > 0 && primaryCount != 1 {
		return nil, ErrInvalidUserAssignment
	}
	return clean, nil
}

func validateAssignmentTargets(departments []domain.Department, positions []domain.Position, assignments []domain.UserAssignment) error {
	departmentByID := make(map[string]domain.Department, len(departments))
	for _, department := range departments {
		departmentByID[department.ID] = department
	}
	positionByID := make(map[string]domain.Position, len(positions))
	for _, position := range positions {
		positionByID[position.ID] = position
	}
	for _, assignment := range assignments {
		department, ok := departmentByID[assignment.DepartmentID]
		if !ok || department.OrganizationID != assignment.OrganizationID || department.Status != "ACTIVE" {
			return ErrInvalidUserAssignment
		}
		if assignment.PositionID == "" {
			continue
		}
		position, ok := positionByID[assignment.PositionID]
		if !ok || position.OrganizationID != assignment.OrganizationID || position.DepartmentID != assignment.DepartmentID || position.Status != "ACTIVE" {
			return ErrInvalidUserAssignment
		}
	}
	return nil
}

func (s *Service) ListRoles(ctx context.Context, orgID string) ([]domain.Role, error) {
	return s.repo.ListRoles(ctx, orgID)
}

func (s *Service) UpdateRoleDataScope(ctx context.Context, actor domain.Principal, orgID, roleKey, scopeType string, departmentIDs []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
	if !actor.HasRole("system_admin") {
		return ErrGrantCeiling
	}
	roleKey = strings.TrimSpace(roleKey)
	scopeType = strings.ToUpper(strings.TrimSpace(scopeType))
	if roleKey == "" || roleKey == "system_admin" {
		return ErrInvalidDataScope
	}
	switch scopeType {
	case domain.DataScopeOrganization, domain.DataScopeDepartment, domain.DataScopeDepartmentTree, domain.DataScopeSelf, domain.DataScopeCustom:
	default:
		return ErrInvalidDataScope
	}
	clean, err := normalizeIDs(departmentIDs, 500)
	if err != nil {
		return ErrInvalidDataScope
	}
	if scopeType == domain.DataScopeCustom {
		if len(clean) == 0 {
			return ErrInvalidDataScope
		}
		departments, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
		if err != nil {
			return err
		}
		allowed := make(map[string]struct{}, len(departments))
		for _, department := range departments {
			if department.Status == "ACTIVE" {
				allowed[department.ID] = struct{}{}
			}
		}
		for _, departmentID := range clean {
			if _, ok := allowed[departmentID]; !ok {
				return ErrInvalidDataScope
			}
		}
	} else if len(clean) != 0 {
		return ErrInvalidDataScope
	}
	roles, err := s.repo.ListRoles(ctx, orgID)
	if err != nil {
		return err
	}
	found := false
	for _, role := range roles {
		if role.Key == roleKey {
			found = true
			break
		}
	}
	if !found {
		return ErrInvalidRole
	}
	return s.repo.ReplaceRoleDataScope(ctx, orgID, roleKey, scopeType, clean)
}

func (s *Service) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *Service) ListMenus(ctx context.Context, principal domain.Principal) ([]domain.Menu, error) {
	menus, err := s.repo.ListMenus(ctx, principal.OrganizationID)
	if err != nil {
		return nil, err
	}
	if principal.HasRole("system_admin") {
		return menus, nil
	}
	filtered := make([]domain.Menu, 0, len(menus))
	for _, menu := range menus {
		if menu.PermissionKey == "" || principal.HasPermission(menu.PermissionKey) {
			filtered = append(filtered, menu)
		}
	}
	return filtered, nil
}

func (s *Service) UpdateMenu(ctx context.Context, actor domain.Principal, orgID, menuKey string, req domain.Menu) (domain.Menu, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Menu{}, err
	}
	menuKey = strings.TrimSpace(menuKey)
	req.ParentKey = strings.TrimSpace(req.ParentKey)
	req.Name = strings.TrimSpace(req.Name)
	req.Route = strings.TrimSpace(req.Route)
	req.Icon = strings.TrimSpace(req.Icon)
	req.PermissionKey = strings.TrimSpace(req.PermissionKey)
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if menuKey == "" || len(menuKey) > 160 || len(req.ParentKey) > 160 || len(req.Name) == 0 || len(req.Name) > 200 || len(req.Route) > 300 || len(req.Icon) > 100 || len(req.PermissionKey) > 160 || req.SortOrder < 0 || (req.Status != "ACTIVE" && req.Status != "DISABLED") || req.ParentKey == menuKey {
		return domain.Menu{}, ErrInvalidMenu
	}
	req.Key = menuKey
	req.OrganizationID = orgID
	return s.repo.UpdateMenu(ctx, orgID, req)
}

func builtinMenus(orgID string) []domain.Menu {
	now := time.Now().UTC()
	return []domain.Menu{
		{OrganizationID: orgID, Key: "dashboard", Name: "工作台", Route: "/dashboard", Icon: "DashboardOutlined", SortOrder: 10, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "portal", Name: "应用门户", Route: "/portal", Icon: "AppstoreOutlined", PermissionKey: "portal.application.read", SortOrder: 15, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "portal.applications", ParentKey: "portal", Name: "应用目录", Route: "/portal/applications", Icon: "AppstoreOutlined", PermissionKey: "portal.application.read", SortOrder: 16, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform", Name: "平台管理", Route: "/group/platform", Icon: "SettingOutlined", SortOrder: 20, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.users", ParentKey: "platform", Name: "用户管理", Route: "/admin/users", Icon: "UserOutlined", PermissionKey: "system.user.read", SortOrder: 21, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.roles", ParentKey: "platform", Name: "角色权限", Route: "/admin/roles", Icon: "SafetyOutlined", PermissionKey: "system.role.read", SortOrder: 22, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.permissions", ParentKey: "platform", Name: "权限清单", Route: "/admin/permissions", Icon: "FileProtectOutlined", PermissionKey: "system.role.read", SortOrder: 23, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.organization", ParentKey: "platform", Name: "组织信息", Route: "/admin/organization", Icon: "ApartmentOutlined", PermissionKey: "system.organization.read", SortOrder: 24, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.departments", ParentKey: "platform", Name: "部门管理", Route: "/admin/departments", Icon: "PartitionOutlined", PermissionKey: "system.department.read", SortOrder: 25, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.positions", ParentKey: "platform", Name: "岗位管理", Route: "/admin/positions", Icon: "IdcardOutlined", PermissionKey: "system.position.read", SortOrder: 26, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "platform.user-groups", ParentKey: "platform", Name: "用户组管理", Route: "/admin/user-groups", Icon: "UsergroupAddOutlined", PermissionKey: "system.user_group.read", SortOrder: 27, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "security", Name: "安全中心", Route: "/group/security", Icon: "SafetyCertificateOutlined", SortOrder: 30, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "security.policy", ParentKey: "security", Name: "安全基线", Route: "/security", Icon: "SafetyCertificateOutlined", PermissionKey: "system.config.read", SortOrder: 31, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "security.sessions", ParentKey: "security", Name: "在线会话", Route: "/admin/sessions", Icon: "LockOutlined", PermissionKey: "system.session.read", SortOrder: 32, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "security.audit-logs", ParentKey: "security", Name: "审计日志", Route: "/admin/audit-logs", Icon: "AuditOutlined", PermissionKey: "system.audit.read", SortOrder: 33, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "governance", Name: "治理中心", Route: "/group/governance", Icon: "CheckSquareOutlined", SortOrder: 40, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "governance.approvals", ParentKey: "governance", Name: "审批中心", Route: "/governance/approvals", Icon: "CheckSquareOutlined", PermissionKey: "approval.request.read", SortOrder: 41, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "governance.temporary-grants", ParentKey: "governance", Name: "临时授权", Route: "/governance/temporary-grants", Icon: "SafetyCertificateOutlined", PermissionKey: "system.temporary_grant.read", SortOrder: 42, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "governance.access-reviews", ParentKey: "governance", Name: "访问复核", Route: "/governance/access-reviews", Icon: "SafetyCertificateOutlined", PermissionKey: "system.access_review.read", SortOrder: 43, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "governance.data-governance", ParentKey: "governance", Name: "数据治理", Route: "/admin/data-governance", Icon: "DatabaseOutlined", PermissionKey: "system.data_policy.read", SortOrder: 44, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "governance.config-changes", ParentKey: "governance", Name: "配置变更", Route: "/admin/config-changes", Icon: "SafetyCertificateOutlined", PermissionKey: "system.config.read", SortOrder: 45, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "operations", Name: "运维中心", Route: "/group/operations", Icon: "SettingOutlined", SortOrder: 50, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
		{OrganizationID: orgID, Key: "operations.status", ParentKey: "operations", Name: "系统状态", Route: "/ops/system", Icon: "SettingOutlined", SortOrder: 51, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now},
	}
}

func (s *Service) UpdateRolePermissions(ctx context.Context, actor domain.Principal, orgID, roleKey string, permissionKeys []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
	roleKey = strings.TrimSpace(roleKey)
	if roleKey == "" || roleKey == "system_admin" {
		return ErrInvalidRole
	}
	allowed := map[string]struct{}{}
	items, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		allowed[item.Key] = struct{}{}
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(permissionKeys))
	for _, key := range permissionKeys {
		key = strings.TrimSpace(key)
		if _, ok := allowed[key]; !ok {
			return ErrInvalidRole
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, key)
	}
	roles, err := s.repo.ListRoles(ctx, orgID)
	if err != nil {
		return err
	}
	var current []string
	found := false
	for _, role := range roles {
		if role.Key != roleKey {
			continue
		}
		found = true
		for _, permission := range role.Permissions {
			current = append(current, permission.Key)
		}
		break
	}
	if !found {
		return ErrInvalidRole
	}
	if err := enforcePermissionMutation(actor, roleKey, current, clean); err != nil {
		return err
	}
	return s.repo.ReplaceRolePermissions(ctx, orgID, roleKey, clean)
}

func (s *Service) UpdateUserRoles(ctx context.Context, actor domain.Principal, orgID, userID string, roleKeys []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
	if len(roleKeys) == 0 {
		roleKeys = []string{"user"}
	}
	roles, err := s.repo.ListRoles(ctx, orgID)
	if err != nil {
		return err
	}
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[role.Key] = struct{}{}
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(roleKeys))
	for _, key := range roleKeys {
		key = strings.TrimSpace(key)
		if _, ok := allowed[key]; !ok {
			return ErrInvalidRole
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, key)
	}
	target, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if target.OrganizationID != orgID {
		return ErrGrantCeiling
	}
	if err := enforceRoleMutation(actor, target.Roles, clean); err != nil {
		return err
	}
	groupRoles, err := s.repo.GroupRolesForUser(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.validateRoleCombination(ctx, orgID, append(clean, groupRoles...)); err != nil {
		return err
	}
	return s.repo.ReplaceUserRoles(ctx, orgID, userID, clean)
}

func (s *Service) validateGroupMembers(ctx context.Context, orgID, groupID string, groupRoles, memberIDs []string) error {
	for _, userID := range memberIDs {
		otherRoles, err := s.repo.RolesForUserExcludingGroup(ctx, userID, groupID)
		if err != nil {
			return err
		}
		if err := s.validateRoleCombination(ctx, orgID, append(otherRoles, groupRoles...)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateRoleCombination(ctx context.Context, orgID string, roleKeys []string) error {
	rules, err := s.repo.RoleConflictRules(ctx, orgID)
	if err != nil {
		return err
	}
	if hasRoleConflict(roleKeys, rules) {
		return ErrRoleConflict
	}
	return nil
}

func hasRoleConflict(roleKeys []string, rules []domain.RoleConflictRule) bool {
	set := make(map[string]struct{}, len(roleKeys))
	for _, key := range roleKeys {
		set[key] = struct{}{}
	}
	for _, rule := range rules {
		_, hasA := set[rule.RoleA]
		_, hasB := set[rule.RoleB]
		if hasA && hasB {
			return true
		}
	}
	return false
}

func authorizeGrantActor(actor domain.Principal, orgID string) error {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" || actor.OrganizationID != orgID {
		return ErrGrantCeiling
	}
	return RequireRecentMFA(actor)
}

func enforceRoleMutation(actor domain.Principal, current, next []string) error {
	if contains(actor.Roles, "system_admin") {
		return nil
	}
	for _, key := range changedValues(current, next) {
		if !contains(actor.Roles, key) {
			return ErrGrantCeiling
		}
	}
	return nil
}

func enforcePermissionMutation(actor domain.Principal, roleKey string, current, next []string) error {
	if contains(actor.Roles, "system_admin") {
		return nil
	}
	if !contains(actor.Roles, roleKey) {
		return ErrGrantCeiling
	}
	for _, key := range changedValues(current, next) {
		if !contains(actor.Permissions, key) {
			return ErrGrantCeiling
		}
	}
	return nil
}

func changedValues(current, next []string) []string {
	before := make(map[string]struct{}, len(current))
	after := make(map[string]struct{}, len(next))
	for _, value := range current {
		before[value] = struct{}{}
	}
	for _, value := range next {
		after[value] = struct{}{}
	}
	changed := make([]string, 0)
	for value := range before {
		if _, ok := after[value]; !ok {
			changed = append(changed, value)
		}
	}
	for value := range after {
		if _, ok := before[value]; !ok {
			changed = append(changed, value)
		}
	}
	return changed
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (s *Service) ListSessions(ctx context.Context, orgID, currentSessionID string) ([]domain.Session, error) {
	items, err := s.repo.ListSessions(ctx, orgID, 500)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Current = items[i].ID == currentSessionID
	}
	return items, nil
}

func (s *Service) RevokeSession(ctx context.Context, orgID, sessionID, currentSessionID string) error {
	if strings.TrimSpace(sessionID) == "" || sessionID == currentSessionID {
		return errors.New("cannot revoke current session")
	}
	return s.repo.RevokeSession(ctx, orgID, sessionID)
}

func (s *Service) SetUserStatus(ctx context.Context, orgID, userID, status string) error {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "ACTIVE" && status != "DISABLED" {
		return errors.New("invalid user status")
	}
	return s.repo.SetUserStatus(ctx, orgID, userID, status)
}

func (s *Service) UnlockUser(ctx context.Context, orgID, userID string) error {
	return s.repo.UnlockUser(ctx, orgID, userID)
}

func (s *Service) AdminResetPassword(ctx context.Context, orgID, userID, next string) error {
	if err := s.enforceOrganizationActive(ctx, orgID); err != nil {
		return err
	}
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return err
	}
	if err := policy.passwordPolicy.Validate(next); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrganizationID != orgID {
		return sql.ErrNoRows
	}
	currentHash, err := s.repo.PasswordHashByID(ctx, userID)
	if err != nil {
		return err
	}
	if s.hasher.Verify(next, currentHash) {
		return ErrPasswordReused
	}
	if policy.history > 0 {
		history, err := s.repo.PasswordHistory(ctx, userID, policy.history)
		if err != nil {
			return err
		}
		for _, old := range history {
			if s.hasher.Verify(next, old) {
				return ErrPasswordReused
			}
		}
	}
	nextHash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.repo.AdminResetPassword(ctx, orgID, userID, currentHash, nextHash)
}
