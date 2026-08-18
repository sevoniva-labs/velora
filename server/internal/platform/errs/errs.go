// Package errs 定义 Velora 业务错误码与错误类型。
//
// 错误码模块（与前端 api/client.ts 中的约定一致）：
//
//	000000 SUCCESS
//	A01xxx AUTH        认证 / OIDC / Session
//	A02xxx APPLICATION 应用领域
//	A03xxx PERMISSION  访问控制
//	A04xxx PORTAL      门户（分类 / 标签 / 收藏 / 设置）
//	A05xxx SYSTEM      系统（内部错误 / 参数 / 数据库）
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Code 为稳定业务错误码（对外返回，禁止把 SQL/堆栈/路径暴露给前端）。
type Code string

const (
	CodeSuccess Code = "000000"

	// A01xxx 认证
	CodeUnauthorized       Code = "A01001" // 未登录或会话失效
	CodeOIDCStateInvalid   Code = "A01002" // OIDC state 校验失败
	CodeOIDCTokenFailed    Code = "A01003" // token 交换失败
	CodeOIDCUserinfoFailed Code = "A01004" // 用户信息获取失败
	CodeOIDCInvalidParam   Code = "A01005" // 登录参数无效
	CodeLoginFailed        Code = "A01006" // 账号密码登录失败（凭据错误）

	// A02xxx 应用
	CodeApplicationNotFound   Code = "A02001"
	CodeApplicationCodeExists Code = "A02002"
	CodeApplicationInvalid    Code = "A02003"
	CodeApplicationDisabled   Code = "A02004" // 应用状态不允许访问/启动

	// A03xxx 权限
	CodeForbidden        Code = "A03001"
	CodePermissionDenied Code = "A03002"

	// A04xxx 门户
	CodeCategoryNotFound    Code = "A04001"
	CodeTagNotFound         Code = "A04002"
	CodeFavoriteNotFound    Code = "A04003"
	CodeCategoryCodeExists  Code = "A04004"
	CodeTagCodeExists       Code = "A04005"
	CodeSettingInvalid      Code = "A04006"
	CodeTodoNotFound        Code = "A04007"
	CodeMailAccountNotFound Code = "A04008"
	CodeMailMessageNotFound Code = "A04009"
	CodeMailSyncFailed      Code = "A04010"
	CodeMailAccountExists   Code = "A04011"

	// A05xxx 系统
	CodeInternal     Code = "A05001"
	CodeDBError      Code = "A05002"
	CodeInvalidParam Code = "A05003"
	CodeRateLimited  Code = "A05004"
	CodeCSRFInvalid  Code = "A05005"
)

// Error 为携带 HTTP 状态码与业务码的错误。
type Error struct {
	Code    Code
	Message string
	Status  int
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// New 构造业务错误。
func New(code Code, status int, message string) *Error {
	return &Error{Code: code, Message: message, Status: status}
}

// Wrap 构造带底层原因的业务错误（原因仅用于日志，不对外返回）。
func Wrap(code Code, status int, message string, err error) *Error {
	return &Error{Code: code, Message: message, Status: status, Err: err}
}

// 常用工厂函数。

func Unauthorized(message string) *Error {
	if message == "" {
		message = "未登录或会话已过期"
	}
	return New(CodeUnauthorized, http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	if message == "" {
		message = "没有权限执行此操作"
	}
	return New(CodeForbidden, http.StatusForbidden, message)
}

func NotFound(code Code, message string) *Error {
	return New(code, http.StatusNotFound, message)
}

func InvalidParam(message string) *Error {
	return New(CodeInvalidParam, http.StatusBadRequest, message)
}

func Internal(message string, err error) *Error {
	return Wrap(CodeInternal, http.StatusInternalServerError, message, err)
}

func DB(err error) *Error {
	return Wrap(CodeDBError, http.StatusInternalServerError, "数据库操作失败", err)
}

// Is 便于 errors.Is 判断。
func Is(err error, code Code) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// StatusOf 提取错误对应的 HTTP 状态码，未知错误按 500。
func StatusOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return http.StatusInternalServerError
}
