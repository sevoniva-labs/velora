package errs

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorBasics(t *testing.T) {
	e := New(CodeApplicationNotFound, http.StatusNotFound, "应用不存在")
	if e.Code != CodeApplicationNotFound || e.Status != http.StatusNotFound {
		t.Fatalf("unexpected error: %+v", e)
	}
	if !errors.Is(e, e) {
		t.Fatal("errors.Is 应识别自身")
	}
	if Is(e, CodeApplicationNotFound) != true {
		t.Fatal("Is 应匹配错误码")
	}
	if Is(e, CodeForbidden) != false {
		t.Fatal("Is 不应匹配其他错误码")
	}
	if StatusOf(e) != http.StatusNotFound {
		t.Fatal("StatusOf 错误")
	}
}

func TestWrapUnwrap(t *testing.T) {
	inner := errors.New("sql: connection refused")
	e := Wrap(CodeDBError, http.StatusInternalServerError, "数据库操作失败", inner)
	if !errors.Is(e, inner) {
		t.Fatal("Wrap 应支持 Unwrap")
	}
	if e.Error() == "" {
		t.Fatal("Error() 不应为空")
	}
}

func TestStatusOfUnknown(t *testing.T) {
	if StatusOf(errors.New("plain")) != http.StatusInternalServerError {
		t.Fatal("未知错误应映射 500")
	}
}

func TestFactories(t *testing.T) {
	if Unauthorized("").Status != http.StatusUnauthorized {
		t.Fatal("Unauthorized 状态码错误")
	}
	if Forbidden("").Status != http.StatusForbidden {
		t.Fatal("Forbidden 状态码错误")
	}
	if InvalidParam("x").Status != http.StatusBadRequest {
		t.Fatal("InvalidParam 状态码错误")
	}
	if Internal("x", nil).Code != CodeInternal {
		t.Fatal("Internal 错误码错误")
	}
	if NotFound(CodeTagNotFound, "x").Code != CodeTagNotFound {
		t.Fatal("NotFound 错误码错误")
	}
}
