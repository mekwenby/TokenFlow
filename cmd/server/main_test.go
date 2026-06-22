package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeShowsPortalInsteadOfRedirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")
	rec := httptest.NewRecorder()

	home(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("home should render a page, got status %d", rec.Code)
	}
	if rec.Header().Get("Location") != "" {
		t.Fatalf("home should not redirect, got Location %q", rec.Header().Get("Location"))
	}
	body := rec.Body.String()
	for _, expected := range []string{`<html lang="zh-CN">`, "普通用户登录", "管理员登录", `href="/account/login"`, `href="/account/register"`, `href="/admin"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in home page:\n%s", expected, body)
		}
	}
}

func TestHomeDefaultsToEnglish(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	home(rec, req)

	body := rec.Body.String()
	for _, expected := range []string{`<html lang="en">`, "User login", "Admin login"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in home page:\n%s", expected, body)
		}
	}
}
