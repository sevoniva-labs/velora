package application

import (
	"context"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// DTO 为应用对外视图。
type DTO struct {
	ID                 uint64       `json:"id"`
	Code               string       `json:"code"`
	Name               string       `json:"name"`
	Description        string       `json:"description"`
	Keywords           string       `json:"keywords,omitempty"`
	Icon               string       `json:"icon"`
	CategoryID         *uint64      `json:"categoryId,omitempty"`
	Category           *CategoryDTO `json:"category,omitempty"`
	SSOType            string       `json:"ssoType"`
	Owner              string       `json:"owner"`
	Department         string       `json:"department"`
	Status             string       `json:"status"`
	Sort               int          `json:"sort"`
	IsFeatured         bool         `json:"isFeatured"`
	HealthCheckEnabled bool         `json:"healthCheckEnabled"`
	HealthStatus       string       `json:"healthStatus,omitempty"`
	Tags               []TagDTO     `json:"tags"`
	Policies           []PolicyDTO  `json:"policies,omitempty"`
	IsFavorite         bool         `json:"isFavorite"`
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
	CreatedBy          string       `json:"createdBy,omitempty"`
	UpdatedBy          string       `json:"updatedBy,omitempty"`

	// 仅管理员视图填充：
	HomeURL                string `json:"homeUrl,omitempty"`
	LaunchURL              string `json:"launchUrl,omitempty"`
	CasdoorApplicationName string `json:"casdoorApplicationName,omitempty"`
	CasdoorClientID        string `json:"casdoorClientId,omitempty"`
	HealthCheckURL         string `json:"healthCheckUrl,omitempty"`
}

// CategoryDTO 为分类精简视图。
type CategoryDTO struct {
	ID   uint64 `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// TagDTO 为标签精简视图。
type TagDTO struct {
	ID   uint64 `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// PolicyDTO 为访问策略视图。
type PolicyDTO struct {
	PolicyType string `json:"policyType"`
	Value      string `json:"value"`
}

// ListFilter 为用户侧应用列表过滤条件。
type ListFilter struct {
	Keyword       string
	CategoryID    uint64
	TagIDs        []uint64
	FeaturedOnly  bool
	FavoritesOnly bool
	Page          int
	PageSize      int
}

// Page 为分页结果。
type Page struct {
	Items    []DTO `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// Input 为应用创建/更新入参。
type Input struct {
	Code                   string      `json:"code"`
	Name                   string      `json:"name"`
	Description            string      `json:"description"`
	Keywords               string      `json:"keywords"`
	Icon                   string      `json:"icon"`
	CategoryID             *uint64     `json:"categoryId"`
	HomeURL                string      `json:"homeUrl"`
	LaunchURL              string      `json:"launchUrl"`
	SSOType                string      `json:"ssoType"`
	CasdoorApplicationName string      `json:"casdoorApplicationName"`
	CasdoorClientID        string      `json:"casdoorClientId"`
	Owner                  string      `json:"owner"`
	Department             string      `json:"department"`
	Status                 string      `json:"status"`
	Sort                   int         `json:"sort"`
	IsFeatured             bool        `json:"isFeatured"`
	HealthCheckEnabled     bool        `json:"healthCheckEnabled"`
	HealthCheckURL         string      `json:"healthCheckUrl"`
	TagIDs                 []uint64    `json:"tagIds"`
	Policies               []PolicyDTO `json:"policies"`
}

// Service 为应用领域服务。
type Service struct {
	repo       *Repository
	db         *gorm.DB
	launch     *LaunchRegistry
	health     *HealthChecker
	adminRole  string
	oidcIssuer string
}

// NewService 创建应用服务。
func NewService(db *gorm.DB, adminRole, oidcIssuer string, checkTimeout time.Duration) *Service {
	repo := NewRepository(db)
	return &Service{
		repo:       repo,
		db:         db,
		launch:     NewLaunchRegistry(oidcIssuer),
		health:     NewHealthChecker(checkTimeout),
		adminRole:  adminRole,
		oidcIssuer: oidcIssuer,
	}
}

// ListPublic 返回当前用户可见的应用（搜索/分类/标签过滤 + 收藏标记 + 分页）。
func (s *Service) ListPublic(ctx context.Context, user *auth.CurrentUser, f ListFilter) (*Page, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 24
	}

	q := s.db.WithContext(ctx).Model(&Application{}).Where("status = ?", StatusEnabled)
	if f.FeaturedOnly {
		q = q.Where("is_featured = ?", true)
	}
	if f.CategoryID > 0 {
		q = q.Where("category_id = ?", f.CategoryID)
	}
	if len(f.TagIDs) > 0 {
		q = q.Where("id IN (SELECT application_id FROM application_tag_relations WHERE tag_id IN ?)", f.TagIDs)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where(
			"name ILIKE ? OR code ILIKE ? OR description ILIKE ? OR keywords ILIKE ? "+
				"OR id IN (SELECT r.application_id FROM application_tag_relations r JOIN application_tags t ON t.id = r.tag_id WHERE t.name ILIKE ?)",
			like, like, like, like, like,
		)
	}

	apps, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 权限过滤（内存执行；MVP 规模合理，规模增长后迁移为 SQL 级过滤）。
	visible := make([]Application, 0, len(apps))
	for _, app := range apps {
		if CanAccess(user, app.Policies) {
			visible = append(visible, app)
		}
	}

	// 收藏状态批量填充。
	favSet, err := s.favoriteSet(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 排序：Featured 优先 → sort → name。
	sort.SliceStable(visible, func(i, j int) bool {
		if visible[i].IsFeatured != visible[j].IsFeatured {
			return visible[i].IsFeatured
		}
		if visible[i].Sort != visible[j].Sort {
			return visible[i].Sort < visible[j].Sort
		}
		return strings.ToLower(visible[i].Name) < strings.ToLower(visible[j].Name)
	})

	total := int64(len(visible))
	start := (f.Page - 1) * f.PageSize
	if start > len(visible) {
		start = len(visible)
	}
	end := start + f.PageSize
	if end > len(visible) {
		end = len(visible)
	}

	items := make([]DTO, 0, end-start)
	for _, app := range visible[start:end] {
		dto := s.toDTO(&app, false)
		dto.IsFavorite = favSet[app.ID]
		items = append(items, *dto)
	}
	return &Page{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// GetPublic 返回单个应用（含可见性校验）。
func (s *Service) GetPublic(ctx context.Context, user *auth.CurrentUser, id uint64) (*DTO, error) {
	app, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != StatusEnabled {
		return nil, errs.NotFound(errs.CodeApplicationNotFound, "应用不存在")
	}
	if !CanAccess(user, app.Policies) {
		return nil, errs.Forbidden("无权访问该应用")
	}
	favSet, err := s.favoriteSet(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	dto := s.toDTO(app, false)
	dto.IsFavorite = favSet[app.ID]
	// 详情页返回健康状态（列表不检查，避免放大请求）。
	dto.HealthStatus = s.health.Check(ctx, app)
	return dto, nil
}

// AdminList 管理员视图：全部应用（含禁用），不做可见性过滤。
func (s *Service) AdminList(ctx context.Context, f ListFilter) (*Page, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	q := s.db.WithContext(ctx).Model(&Application{})
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}
	if f.CategoryID > 0 {
		q = q.Where("category_id = ?", f.CategoryID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, errs.DB(err)
	}
	apps, err := s.repo.List(ctx, q.Order("sort ASC, id DESC").Offset((f.Page-1)*f.PageSize).Limit(f.PageSize))
	if err != nil {
		return nil, err
	}
	items := make([]DTO, 0, len(apps))
	for i := range apps {
		items = append(items, *s.toDTO(&apps[i], true))
	}
	return &Page{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Create 创建应用（管理员）。
func (s *Service) Create(ctx context.Context, operator string, in Input) (*DTO, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByCode(ctx, strings.TrimSpace(in.Code))
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errs.New(errs.CodeApplicationCodeExists, 409, "应用编码已存在")
	}
	app := &Application{
		Code:                   strings.TrimSpace(in.Code),
		Name:                   strings.TrimSpace(in.Name),
		Description:            in.Description,
		Keywords:               in.Keywords,
		Icon:                   in.Icon,
		CategoryID:             in.CategoryID,
		HomeURL:                in.HomeURL,
		LaunchURL:              in.LaunchURL,
		SSOType:                normalizeSSOType(in.SSOType),
		CasdoorApplicationName: in.CasdoorApplicationName,
		CasdoorClientID:        in.CasdoorClientID,
		Owner:                  in.Owner,
		Department:             in.Department,
		Status:                 normalizeStatus(in.Status),
		Sort:                   in.Sort,
		IsFeatured:             in.IsFeatured,
		HealthCheckEnabled:     in.HealthCheckEnabled,
		HealthCheckURL:         in.HealthCheckURL,
		CreatedBy:              operator,
		UpdatedBy:              operator,
	}
	if err := s.repo.Create(ctx, app); err != nil {
		return nil, err
	}
	if len(in.TagIDs) > 0 {
		if err := s.repo.ReplaceTags(ctx, app.ID, in.TagIDs); err != nil {
			return nil, err
		}
	}
	if len(in.Policies) > 0 {
		if err := s.repo.ReplacePolicies(ctx, app.ID, toPolicyModels(in.Policies)); err != nil {
			return nil, err
		}
	}
	fresh, err := s.repo.Get(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	return s.toDTO(fresh, true), nil
}

// Update 更新应用（管理员）。
func (s *Service) Update(ctx context.Context, operator string, id uint64, in Input) (*DTO, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	app, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	// 编码唯一性（排除自身）。
	existing, err := s.repo.GetByCode(ctx, strings.TrimSpace(in.Code))
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != id {
		return nil, errs.New(errs.CodeApplicationCodeExists, 409, "应用编码已存在")
	}

	app.Code = strings.TrimSpace(in.Code)
	app.Name = strings.TrimSpace(in.Name)
	app.Description = in.Description
	app.Keywords = in.Keywords
	app.Icon = in.Icon
	app.CategoryID = in.CategoryID
	app.HomeURL = in.HomeURL
	app.LaunchURL = in.LaunchURL
	app.SSOType = normalizeSSOType(in.SSOType)
	app.CasdoorApplicationName = in.CasdoorApplicationName
	app.CasdoorClientID = in.CasdoorClientID
	app.Owner = in.Owner
	app.Department = in.Department
	app.Status = normalizeStatus(in.Status)
	app.Sort = in.Sort
	app.IsFeatured = in.IsFeatured
	app.HealthCheckEnabled = in.HealthCheckEnabled
	app.HealthCheckURL = in.HealthCheckURL
	app.UpdatedBy = operator

	if err := s.repo.Update(ctx, app); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceTags(ctx, app.ID, in.TagIDs); err != nil {
		return nil, err
	}
	if err := s.repo.ReplacePolicies(ctx, app.ID, toPolicyModels(in.Policies)); err != nil {
		return nil, err
	}
	fresh, err := s.repo.Get(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	return s.toDTO(fresh, true), nil
}

// Delete 删除应用（管理员）。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// SetPolicies 更新应用访问策略（管理员）。
func (s *Service) SetPolicies(ctx context.Context, id uint64, policies []PolicyDTO) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	for _, p := range policies {
		if !ValidPolicyType(p.PolicyType) {
			return errs.InvalidParam("不支持的策略类型: " + p.PolicyType)
		}
		if p.PolicyType != PolicyTypeEveryone && strings.TrimSpace(p.Value) == "" {
			return errs.InvalidParam("策略 " + p.PolicyType + " 需要指定取值")
		}
	}
	return s.repo.ReplacePolicies(ctx, id, toPolicyModels(policies))
}

// toDTO 转换（isAdmin 控制敏感字段是否输出）。
func (s *Service) toDTO(app *Application, isAdmin bool) *DTO {
	dto := &DTO{
		ID:                 app.ID,
		Code:               app.Code,
		Name:               app.Name,
		Description:        app.Description,
		Keywords:           app.Keywords,
		Icon:               app.Icon,
		CategoryID:         app.CategoryID,
		SSOType:            app.SSOType,
		Owner:              app.Owner,
		Department:         app.Department,
		Status:             app.Status,
		Sort:               app.Sort,
		IsFeatured:         app.IsFeatured,
		HealthCheckEnabled: app.HealthCheckEnabled,
		HealthStatus:       HealthUnknown,
		CreatedAt:          app.CreatedAt,
		UpdatedAt:          app.UpdatedAt,
		CreatedBy:          app.CreatedBy,
		UpdatedBy:          app.UpdatedBy,
	}
	if app.Category != nil {
		dto.Category = &CategoryDTO{ID: app.Category.ID, Code: app.Category.Code, Name: app.Category.Name}
	}
	for _, t := range app.Tags {
		dto.Tags = append(dto.Tags, TagDTO{ID: t.ID, Code: t.Code, Name: t.Name})
	}
	for _, p := range app.Policies {
		dto.Policies = append(dto.Policies, PolicyDTO{PolicyType: p.PolicyType, Value: p.Value})
	}
	if isAdmin {
		dto.HomeURL = app.HomeURL
		dto.LaunchURL = app.LaunchURL
		dto.CasdoorApplicationName = app.CasdoorApplicationName
		dto.CasdoorClientID = app.CasdoorClientID
		dto.HealthCheckURL = app.HealthCheckURL
	}
	return dto
}

// favoriteSet 返回用户收藏的应用 ID 集合。
func (s *Service) favoriteSet(ctx context.Context, userID string) (map[uint64]bool, error) {
	rows, err := s.db.WithContext(ctx).Table("application_favorites").
		Select("application_id").
		Where("user_id = ?", userID).
		Rows()
	if err != nil {
		return nil, errs.DB(err)
	}
	defer rows.Close()
	set := map[uint64]bool{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, errs.DB(err)
		}
		set[id] = true
	}
	return set, rows.Err()
}

func (s *Service) tagsFor(ctx context.Context, appID uint64) ([]TagDTO, error) {
	var tags []TagDTO
	err := s.db.WithContext(ctx).Raw(
		`SELECT t.id, t.code, t.name FROM application_tags t
		 JOIN application_tag_relations r ON r.tag_id = t.id
		 WHERE r.application_id = ? ORDER BY t.sort ASC, t.id ASC`, appID,
	).Scan(&tags).Error
	if err != nil {
		return nil, errs.DB(err)
	}
	return tags, nil
}

func (s *Service) policiesFor(ctx context.Context, appID uint64) ([]AccessPolicy, error) {
	var policies []AccessPolicy
	if err := s.db.WithContext(ctx).Where("application_id = ?", appID).Order("id ASC").Find(&policies).Error; err != nil {
		return nil, errs.DB(err)
	}
	return policies, nil
}

// --- 校验与工具 ---

func validateInput(in Input) error {
	if strings.TrimSpace(in.Code) == "" {
		return errs.InvalidParam("应用编码不能为空")
	}
	if strings.TrimSpace(in.Name) == "" {
		return errs.InvalidParam("应用名称不能为空")
	}
	if in.SSOType == "" {
		in.SSOType = SSOTypeURL
	}
	switch in.SSOType {
	case SSOTypeURL, SSOTypeOIDC, SSOTypeSAML, SSOTypeCAS, SSOTypeForwardAuth:
	default:
		return errs.InvalidParam("不支持的 SSO 接入类型: " + in.SSOType)
	}
	if in.SSOType == SSOTypeOIDC {
		if strings.TrimSpace(in.CasdoorClientID) == "" {
			return errs.InvalidParam("OIDC 应用需要配置 Casdoor Client ID")
		}
	}
	if err := validateURLField(in.HomeURL, "应用主页地址"); err != nil {
		return err
	}
	if err := validateURLField(in.LaunchURL, "启动地址"); err != nil {
		return err
	}
	if err := validateURLField(in.HealthCheckURL, "健康检查地址"); err != nil {
		return err
	}
	return nil
}

func normalizeSSOType(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case SSOTypeOIDC:
		return SSOTypeOIDC
	case SSOTypeSAML:
		return SSOTypeSAML
	case SSOTypeCAS:
		return SSOTypeCAS
	case SSOTypeForwardAuth:
		return SSOTypeForwardAuth
	default:
		return SSOTypeURL
	}
}

func normalizeStatus(s string) string {
	if strings.ToUpper(strings.TrimSpace(s)) == StatusDisabled {
		return StatusDisabled
	}
	return StatusEnabled
}

func toPolicyModels(dtos []PolicyDTO) []AccessPolicy {
	out := make([]AccessPolicy, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, AccessPolicy{
			PolicyType: d.PolicyType,
			Value:      d.Value,
		})
	}
	return out
}

// ValidPolicyType 校验策略类型。
func ValidPolicyType(t string) bool {
	switch t {
	case PolicyTypeEveryone, PolicyTypeOrganization, PolicyTypeRole, PolicyTypeGroup, PolicyTypeUser:
		return true
	}
	return false
}
