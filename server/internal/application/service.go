package application

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/casdoor"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/tag"
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
	IsNew              bool         `json:"isNew"`
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
	// Cursor keyset 游标（Phase D2）：形如 "featured|sort|nameLower|id" 的 base64 串。
	// 提供时按复合键继续向后翻页（替代 OFFSET）；为空则使用 Page/PageSize。
	Cursor string
}

// Page 为分页结果。
type Page struct {
	Items    []DTO `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	// NextCursor 下一页游标（keyset 分页时返回，供继续翻页）。
	NextCursor string `json:"nextCursor,omitempty"`
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
	settings   SettingsReader
	// oidcClientFactory 为 VELORA_OIDC 应用创建 OIDC client 的回调（由组装层注入，
	// 避免 application 包反向依赖 oidcprovider）。返回 clientID + 明文 secret。
	oidcClientFactory func(ctx context.Context, applicationID uint64, redirectURIs []string) (clientID, clientSecret string, err error)
}

// NewService 创建应用服务。
// publicBaseURL 为 Velora 对外地址（VELORA_OIDC issuer）；
// clientResolver 按应用 ID 解析 Velora OIDC client（由组装层注入，可为 nil）；
// oidcClientFactory 为 VELORA_OIDC 应用自动生成 client（可为 nil，则需管理员手动配置）。
func NewService(db *gorm.DB, adminRole, oidcIssuer, publicBaseURL string, checkTimeout time.Duration, clientResolver func(ctx context.Context, applicationID uint64) (clientID string, redirectURIs []string, ok bool, err error), oidcClientFactory func(ctx context.Context, applicationID uint64, redirectURIs []string) (clientID, clientSecret string, err error)) *Service {
	repo := NewRepository(db)
	return &Service{
		repo:              repo,
		db:                db,
		launch:            NewLaunchRegistry(oidcIssuer, publicBaseURL, clientResolver),
		health:            NewHealthChecker(checkTimeout),
		adminRole:         adminRole,
		oidcIssuer:        oidcIssuer,
		oidcClientFactory: oidcClientFactory,
	}
}

// SettingsReader 为门户设置读取器（portal.Service 满足该接口）。
type SettingsReader interface {
	Get(ctx context.Context, key string) (string, error)
}

// SetSettingsReader 注入门户设置读取器（可选；未注入时「新」标识按默认 7 天）。
func (s *Service) SetSettingsReader(r SettingsReader) {
	s.settings = r
}

// newBadgeCutoff 计算「新」应用标识的创建时间下限：createdAt 晚于该时间即为新应用。
// 天数取门户设置 new_badge_days，默认 7；0 表示关闭标识；非法值回退默认。
func (s *Service) newBadgeCutoff(ctx context.Context) time.Time {
	days := 7
	if s.settings != nil {
		if v, err := s.settings.Get(ctx, "new_badge_days"); err == nil {
			if n, convErr := strconv.Atoi(strings.TrimSpace(v)); convErr == nil && n >= 0 && n <= 90 {
				days = n
			}
		}
	}
	return time.Now().AddDate(0, 0, -days)
}

// ListPublic 返回当前用户可见的应用（搜索/分类/标签过滤 + 收藏标记 + 分页）。
// Phase D2：权限过滤下推 SQL（accessSQL），分页支持 keyset 游标（Cursor）或 OFFSET。
func (s *Service) ListPublic(ctx context.Context, user *auth.CurrentUser, f ListFilter) (*Page, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 24
	} else if f.PageSize > 100 {
		f.PageSize = 100
	}

	q := s.db.WithContext(ctx).Model(&Application{}).Where("status = ?", StatusEnabled)
	if f.FeaturedOnly {
		q = q.Where("is_featured = ?", true)
	}
	if f.FavoritesOnly {
		// 仅当前用户的收藏（应用收藏表）。
		q = q.Where("id IN (SELECT application_id FROM application_favorites WHERE user_id = ?)", user.ID)
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
	// 权限过滤下推 SQL（Phase D2）。
	accessCond, accessArgs := accessSQL(user)
	q = q.Where(accessCond, accessArgs...)

	// keyset 游标：按 (is_featured, sort, name, id) 复合键续页。
	cursor, err := decodeListCursor(f.Cursor)
	if err != nil {
		return nil, errs.InvalidParam("游标无效")
	}
	if cursor != nil {
		q = q.Where(
			`(is_featured < ?) OR
			 (is_featured = ? AND (sort > ? OR (sort = ? AND (LOWER(name) > ? OR (LOWER(name) = ? AND id > ?)))))`,
			cursor.Featured, cursor.Featured, cursor.Sort, cursor.Sort, cursor.NameLower, cursor.NameLower, cursor.ID,
		)
	}

	// 总数（过滤后）。
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, errs.DB(err)
	}

	// 排序与分页（keyset 时 limit 取 pageSize+1 判断是否还有下一页）。
	limit := f.PageSize
	q = q.Order("is_featured DESC").Order("sort ASC").Order("LOWER(name) ASC").Order("id ASC")
	if cursor == nil {
		q = q.Offset((f.Page - 1) * f.PageSize)
	} else {
		limit++
	}
	q = q.Limit(limit)

	var apps []Application
	if err := q.Find(&apps).Error; err != nil {
		return nil, errs.DB(err)
	}

	// keyset：多取一条判定下一页；OFFSET：SQL 已分页，无需再切。
	nextCursor := ""
	if cursor != nil {
		if len(apps) > f.PageSize {
			apps = apps[:f.PageSize]
			last := apps[len(apps)-1]
			nextCursor = encodeListCursor(last.IsFeatured, last.Sort, strings.ToLower(last.Name), last.ID)
		}
	}

	// 收藏状态批量填充。
	favSet, err := s.favoriteSet(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	cutoff := s.newBadgeCutoff(ctx)
	items := make([]DTO, 0, len(apps))
	for _, app := range apps {
		dto := s.toDTO(&app, false)
		dto.IsFavorite = favSet[app.ID]
		dto.IsNew = app.CreatedAt.After(cutoff)
		items = append(items, *dto)
	}
	return &Page{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize, NextCursor: nextCursor}, nil
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
	dto.IsNew = app.CreatedAt.After(s.newBadgeCutoff(ctx))
	// 详情页返回健康状态（列表不检查，避免放大请求）。
	dto.HealthStatus = s.health.Check(ctx, app)
	return dto, nil
}

// AdminList 管理员视图：全部应用（含禁用），不做可见性过滤。
func (s *Service) AdminList(ctx context.Context, f ListFilter) (*Page, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	} else if f.PageSize > 100 {
		f.PageSize = 100
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
	if err := s.validateTagIDs(ctx, in.TagIDs); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, app); err != nil {
		return nil, err
	}
	// VELORA_OIDC 应用：自动生成 OIDC client（回调由组装层注入；生成失败不阻塞应用创建，
	// 管理员可稍后在管理后台手动生成）。
	if app.SSOType == SSOTypeVeloraOIDC && s.oidcClientFactory != nil {
		redirects := []string{}
		if strings.TrimSpace(app.LaunchURL) != "" {
			redirects = append(redirects, strings.TrimSpace(app.LaunchURL))
		}
		if _, _, err := s.oidcClientFactory(ctx, app.ID, redirects); err != nil {
			slog.Warn("VELORA_OIDC 应用自动生成 client 失败", "appID", app.ID, "err", err)
		}
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

	if err := s.validateTagIDs(ctx, in.TagIDs); err != nil {
		return nil, err
	}
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

// validateTagIDs 校验标签 ID 全部存在，防止外键错误变成 500。
func (s *Service) validateTagIDs(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&tag.Tag{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return errs.DB(err)
	}
	if count != int64(len(ids)) {
		return errs.New(errs.CodeTagNotFound, http.StatusBadRequest, "存在无效的标签 ID")
	}
	return nil
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
	// 保证 JSON 输出空数组而非 null（前端 .map() 直接消费）。
	dto.Tags = make([]TagDTO, 0, len(app.Tags))
	for _, t := range app.Tags {
		dto.Tags = append(dto.Tags, TagDTO{ID: t.ID, Code: t.Code, Name: t.Name})
	}
	// 策略（组织/角色/组名）仅管理员可见，普通用户只需可见性结果。
	if isAdmin {
		for _, p := range app.Policies {
			dto.Policies = append(dto.Policies, PolicyDTO{PolicyType: p.PolicyType, Value: p.Value})
		}
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
// listCursor keyset 游标内容（Phase D2）。
type listCursor struct {
	Featured  bool
	Sort      int
	NameLower string
	ID        uint64
}

// encodeListCursor 编码游标：base64("featured|sort|nameLower|id")。
func encodeListCursor(featured bool, sort int, nameLower string, id uint64) string {
	raw := fmt.Sprintf("%t|%d|%s|%d", featured, sort, nameLower, id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeListCursor 解码游标；空串或非法返回 nil（走 OFFSET 分页）。
func decodeListCursor(cursor string) (*listCursor, error) {
	if cursor == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("游标解码失败")
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("游标格式错误")
	}
	featured, err := strconv.ParseBool(parts[0])
	if err != nil {
		return nil, fmt.Errorf("游标 featured 无效")
	}
	sortVal, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("游标 sort 无效")
	}
	id, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("游标 id 无效")
	}
	return &listCursor{Featured: featured, Sort: sortVal, NameLower: parts[2], ID: id}, nil
}

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
	case SSOTypeURL, SSOTypeOIDC, SSOTypeSAML, SSOTypeCAS, SSOTypeForwardAuth, SSOTypeVeloraOIDC:
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
	case SSOTypeVeloraOIDC:
		return SSOTypeVeloraOIDC
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

// SyncFromCasdoor 将 Casdoor 应用（OIDC 客户端）同步为门户应用：
//   - 按 casdoor_client_id 匹配，已存在则刷新名称/图标/主页/描述（保留分类、策略、收藏、标签等门户配置）
//   - 不存在则创建（默认启用、EVERYONE 可见、SSO 类型 OIDC）
//
// 同步只写入 Velora 侧数据，不修改 Casdoor 任何内容。
func (s *Service) SyncFromCasdoor(ctx context.Context, operator string, apps []casdoor.SyncedApplication) (created, updated int, err error) {
	for _, in := range apps {
		if strings.TrimSpace(in.DisplayName) == "" && strings.TrimSpace(in.Name) == "" {
			continue
		}
		existing, gerr := s.repo.GetByCasdoorClientID(ctx, in.ClientID)
		if gerr != nil {
			return created, updated, gerr
		}

		name := strings.TrimSpace(in.DisplayName)
		if name == "" {
			name = in.Name
		}
		icon := strings.TrimSpace(in.Logo)
		home := strings.TrimSpace(in.HomepageURL)

		if existing != nil {
			existing.Name = name
			if icon != "" {
				existing.Icon = icon
			}
			if home != "" {
				existing.HomeURL = home
			}
			if in.Description != "" {
				existing.Description = in.Description
			}
			existing.SSOType = SSOTypeOIDC
			existing.UpdatedBy = operator
			if uerr := s.repo.Update(ctx, existing); uerr != nil {
				return created, updated, uerr
			}
			updated++
			continue
		}

		code := syncCode(in.Name)
		if code == "" {
			code = syncCode(name)
		}
		// 编码冲突时追加 client id 片段保证唯一。
		if conflict, cerr := s.repo.GetByCode(ctx, code); cerr != nil {
			return created, updated, cerr
		} else if conflict != nil {
			suffix := in.ClientID
			if len(suffix) > 6 {
				suffix = suffix[:6]
			}
			code = code + "-" + suffix
		}

		app := &Application{
			Code:                   code,
			Name:                   name,
			Description:            in.Description,
			Icon:                   icon,
			HomeURL:                home,
			SSOType:                SSOTypeOIDC,
			CasdoorApplicationName: in.Name,
			CasdoorClientID:        in.ClientID,
			Status:                 StatusEnabled,
			CreatedBy:              operator,
			UpdatedBy:              operator,
		}
		if cerr := s.repo.Create(ctx, app); cerr != nil {
			return created, updated, cerr
		}
		// 新同步应用默认 EVERYONE 可见，管理员后续可调整。
		if perr := s.repo.ReplacePolicies(ctx, app.ID, []AccessPolicy{{PolicyType: PolicyTypeEveryone}}); perr != nil {
			return created, updated, perr
		}
		created++
	}
	return created, updated, nil
}

// syncCode 将 Casdoor 应用标识转换为门户应用编码（小写字母数字连字符）。
func syncCode(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
