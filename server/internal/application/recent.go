package application

import (
	"context"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// RecentItem 为「最近使用」条目。
type RecentItem struct {
	Application   *DTO      `json:"application"`
	LastVisitedAt time.Time `json:"lastVisitedAt"`
	VisitCount    int64     `json:"visitCount"`
}

// RecentForUser 返回当前用户最近使用的应用（按 last_visited_at 倒序）。
// 仅包含当前可见且 ENABLED 的应用；limit 为返回条数上限。
func (s *Service) RecentForUser(ctx context.Context, user *auth.CurrentUser, limit int) ([]RecentItem, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 50 {
		limit = 50
	}

	type row struct {
		ApplicationID uint64
		VisitCount    int64
		LastVisitedAt time.Time
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(`
		SELECT v.application_id, v.visit_count, v.last_visited_at
		FROM application_visits v
		JOIN applications a ON a.id = v.application_id
		WHERE v.user_id = ? AND a.status = 'ENABLED'
		ORDER BY v.last_visited_at DESC
		LIMIT ?`, user.ID, limit*3,
	).Scan(&rows).Error; err != nil {
		return nil, errs.DB(err)
	}

	cutoff := s.newBadgeCutoff(ctx)
	items := make([]RecentItem, 0, len(rows))
	for _, r := range rows {
		if len(items) >= limit {
			break
		}
		app, err := s.repo.Get(ctx, r.ApplicationID)
		if err != nil {
			// 应用已删除等场景跳过。
			continue
		}
		if !CanAccess(user, app.Policies) {
			continue
		}
		dto := s.toDTO(app, false)
		dto.IsNew = app.CreatedAt.After(cutoff)
		items = append(items, RecentItem{
			Application:   dto,
			LastVisitedAt: r.LastVisitedAt,
			VisitCount:    r.VisitCount,
		})
	}
	return items, nil
}

// PopularForUser 返回热门应用（按全部用户访问量倒序，当前用户可见）。
func (s *Service) PopularForUser(ctx context.Context, user *auth.CurrentUser, limit int) ([]*DTO, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 50 {
		limit = 50
	}
	type row struct {
		ApplicationID uint64
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(`
		SELECT v.application_id
		FROM application_visits v
		JOIN applications a ON a.id = v.application_id
		WHERE a.status = 'ENABLED'
		GROUP BY v.application_id
		ORDER BY SUM(v.visit_count) DESC
		LIMIT ?`, limit*3,
	).Scan(&rows).Error; err != nil {
		return nil, errs.DB(err)
	}
	cutoff := s.newBadgeCutoff(ctx)
	items := make([]*DTO, 0, len(rows))
	for _, r := range rows {
		if len(items) >= limit {
			break
		}
		app, err := s.repo.Get(ctx, r.ApplicationID)
		if err != nil {
			continue
		}
		if !CanAccess(user, app.Policies) {
			continue
		}
		dto := s.toDTO(app, false)
		dto.IsNew = app.CreatedAt.After(cutoff)
		items = append(items, dto)
	}
	return items, nil
}
