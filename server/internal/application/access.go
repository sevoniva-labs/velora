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
