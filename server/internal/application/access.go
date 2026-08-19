package application

import (
	"net/url"
	"strings"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// CanAccess 判断用户是否可访问该应用（按访问策略）。
// 无任何策略时默认可访问（EVERYONE 语义），保证第一阶段开箱即用。
func CanAccess(user *auth.CurrentUser, policies []AccessPolicy) bool {
	if user == nil {
		return false
	}
	if len(policies) == 0 {
		return true
	}
	for _, p := range policies {
		switch p.PolicyType {
		case PolicyTypeEveryone:
			return true
		case PolicyTypeOrganization:
			if p.Value != "" && user.Organization != "" && user.Organization == p.Value {
				return true
			}
		case PolicyTypeRole:
			if p.Value != "" && contains(user.Roles, p.Value) {
				return true
			}
		case PolicyTypeGroup:
			if p.Value != "" && contains(user.Groups, p.Value) {
				return true
			}
		case PolicyTypeUser:
			if p.Value != "" && user.ID == p.Value {
				return true
			}
		}
	}
	return false
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// accessSQL 生成 SQL 级权限过滤条件（Phase D2：替代内存过滤，分页下推数据库）。
// 语义与 CanAccess 一致：无策略 → 可见；有任一匹配策略 → 可见。
// 返回 (sqlFragment, args)；sqlFragment 可直接嵌入 WHERE（带占位符）。
func accessSQL(user *auth.CurrentUser) (string, []any) {
	if user == nil {
		// 无用户：仅可见"无策略"或"EVERYONE"应用（通常不会走到，防御性）。
		return `(NOT EXISTS (SELECT 1 FROM application_access_policies p WHERE p.application_id = applications.id)
		         OR EXISTS (SELECT 1 FROM application_access_policies p
		                    WHERE p.application_id = applications.id AND p.policy_type = 'EVERYONE'))`, nil
	}
	args := []any{}
	// 显式构造（条件与实参顺序一一对应）
	var conds []string
	conds = append(conds, "p.policy_type = 'EVERYONE'")

	args = append(args, user.Organization)
	conds = append(conds, "(p.policy_type = 'ORGANIZATION' AND p.value = ?)")

	if len(user.Roles) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(user.Roles)), ",")
		conds = append(conds, "(p.policy_type = 'ROLE' AND p.value IN ("+ph+"))")
		for _, r := range user.Roles {
			args = append(args, r)
		}
	}
	if len(user.Groups) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(user.Groups)), ",")
		conds = append(conds, "(p.policy_type = 'GROUP' AND p.value IN ("+ph+"))")
		for _, g := range user.Groups {
			args = append(args, g)
		}
	}
	args = append(args, user.ID)
	conds = append(conds, "(p.policy_type = 'USER' AND p.value = ?)")

	return `(NOT EXISTS (SELECT 1 FROM application_access_policies p WHERE p.application_id = applications.id)
	        OR EXISTS (SELECT 1 FROM application_access_policies p
	                   WHERE p.application_id = applications.id AND (` + strings.Join(conds, " OR ") + `)))`, args
}

// validateURLField 校验 URL 字段：允许空（非必填），否则必须是 http/https 且含主机名。
// 用于拦截 path traversal / 注入式 URL / 任意 scheme（如 file://、javascript:）。
func validateURLField(raw, field string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errs.InvalidParam(field + " 不是合法 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errs.InvalidParam(field + " 仅支持 http/https")
	}
	if u.Host == "" {
		return errs.InvalidParam(field + " 缺少主机名")
	}
	return nil
}
