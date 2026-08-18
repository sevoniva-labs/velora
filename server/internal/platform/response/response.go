// Package response 提供 Velora 统一返回结构：
//
//	{"code":"000000","message":"success","data":{...},"requestId":"..."}
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Body 为统一响应体。
type Body struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"requestId"`
}

// OK 返回成功响应（data 可为 nil）。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:      string(errs.CodeSuccess),
		Message:   "success",
		Data:      data,
		RequestID: RequestID(c),
	})
}

// Created 返回 201 响应。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{
		Code:      string(errs.CodeSuccess),
		Message:   "success",
		Data:      data,
		RequestID: RequestID(c),
	})
}

// NoContent 返回 204。
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error 根据业务错误写出统一错误响应；未知错误降级为 500 内部错误。
func Error(c *gin.Context, err error) {
	var e *errs.Error
	if !asErr(err, &e) {
		e = errs.Internal("服务内部错误", err)
	}
	c.JSON(e.Status, Body{
		Code:      string(e.Code),
		Message:   e.Message,
		RequestID: RequestID(c),
	})
}

// ErrorWith 直接以指定错误码/状态/消息写错误响应（用于非 *errs.Error 场景）。
func ErrorWith(c *gin.Context, status int, code errs.Code, message string) {
	c.JSON(status, Body{
		Code:      string(code),
		Message:   message,
		RequestID: RequestID(c),
	})
}

const requestIDKey = "velora.request_id"

// SetRequestID 由中间件调用，把 request id 存入上下文。
func SetRequestID(c *gin.Context, id string) {
	c.Set(requestIDKey, id)
}

// RequestID 返回当前请求的 request id（供审计等模块复用）。
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func asErr(err error, target **errs.Error) bool {
	// 用 errors.As：支持被 %w 包装的业务错误，避免降级为 500。
	if errors.As(err, target) {
		return true
	}
	return false
}
