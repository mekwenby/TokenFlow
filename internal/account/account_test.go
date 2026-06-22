package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"tokenflow/internal/store"
)

func TestRegisterLoginAndConsumerKeys(t *testing.T) {
	handler, router := testAccount(t)

	form := url.Values{"email": {"USER@example.com"}, "password": {"password123"}}
	req := httptest.NewRequest(http.MethodPost, "/account/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Registration submitted") {
		t.Fatalf("unexpected register response: %d %s", rec.Code, rec.Body.String())
	}
	user, err := handler.store.ConsumerUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.Status != store.ConsumerStatusPending {
		t.Fatalf("new user should be pending: %#v", user)
	}

	req = httptest.NewRequest(http.MethodPost, "/account/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "already registered") {
		t.Fatalf("duplicate register was not rejected: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "not enabled") {
		t.Fatalf("pending login should be rejected: %d %s", rec.Code, rec.Body.String())
	}

	user, err = handler.store.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, 100)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/account/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account" {
		t.Fatalf("enabled login should redirect: %d %s", rec.Code, rec.Body.String())
	}
	cookies, csrf := accountCookies(rec)
	if csrf == "" {
		t.Fatalf("csrf cookie was not set: %#v", rec.Result().Cookies())
	}

	req = httptest.NewRequest(http.MethodGet, "/account/api/keys", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("unexpected empty keys response: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/api/keys", strings.NewReader(`{"name":"app"}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("key create without csrf should be rejected: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/api/keys", strings.NewReader(`{"name":"app"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected create key status: %d %s", rec.Code, rec.Body.String())
	}
	var key store.DistributionKey
	if err := json.Unmarshal(rec.Body.Bytes(), &key); err != nil {
		t.Fatal(err)
	}
	if key.PlainKey == "" || key.ConsumerUserID == nil || *key.ConsumerUserID != user.ID {
		t.Fatalf("unexpected consumer key: %#v", key)
	}

	req = httptest.NewRequest(http.MethodPut, "/account/api/keys", strings.NewReader(`{"id":`+strconv.FormatInt(key.ID, 10)+`,"name":"renamed","enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected update key status: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &key); err != nil {
		t.Fatal(err)
	}
	if key.Name != "renamed" || key.Enabled {
		t.Fatalf("key was not updated: %#v", key)
	}

	other, err := handler.store.CreateConsumerUser(context.Background(), "other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err = handler.store.UpdateConsumerUser(context.Background(), other.ID, store.ConsumerStatusEnabled, 100)
	if err != nil {
		t.Fatal(err)
	}
	otherRec := httptest.NewRecorder()
	if err := handler.sessions.Create(otherRec, other.ID, other.Email); err != nil {
		t.Fatal(err)
	}
	otherCookies, otherCSRF := accountCookies(otherRec)
	req = httptest.NewRequest(http.MethodDelete, "/account/api/keys?id="+strconv.FormatInt(key.ID, 10), nil)
	req.Header.Set("X-CSRF-Token", otherCSRF)
	for _, cookie := range otherCookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other user should not delete key: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAccountPagesUseLanguageAndSharedScripts(t *testing.T) {
	handler, router := testAccount(t)

	req := httptest.NewRequest(http.MethodGet, "/account/register", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `<html lang="zh-CN">`) || !strings.Contains(body, "创建账号") || !strings.Contains(body, "注册") {
		t.Fatalf("expected Chinese register page, got status=%d body=%s", rec.Code, body)
	}

	req = httptest.NewRequest(http.MethodGet, "/account/login", nil)
	req.Header.Set("Accept-Language", "zh-CN")
	req.AddCookie(&http.Cookie{Name: adminLangCookie, Value: "en"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, "Account login") {
		t.Fatalf("expected English login page from language cookie, got status=%d body=%s", rec.Code, body)
	}

	user, err := handler.store.CreateConsumerUser(context.Background(), "viewer@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	user, err = handler.store.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, 1000)
	if err != nil {
		t.Fatal(err)
	}
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, user.ID, user.Email); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodGet, "/account", nil)
	req.AddCookie(&http.Cookie{Name: adminLangCookie, Value: "zh-CN"})
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == sessionCookie || cookie.Name == csrfCookie {
			req.AddCookie(cookie)
		}
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body = rec.Body.String()
	for _, expected := range []string{`<html lang="zh-CN">`, "使用情况", "API 地址", "创建 Key", `window.__ACCOUNT_LANG__ = "zh-CN"`, `window.__ACCOUNT_I18N__`, `"copied":"已复制"`, "common.js", "account.js"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in account dashboard:\n%s", expected, body)
		}
	}
}

func TestAccountTranslationsIncludeChineseKeys(t *testing.T) {
	for key := range accountTranslations["en"] {
		if _, ok := accountTranslations["zh-CN"][key]; !ok {
			t.Fatalf("missing zh-CN account translation for %q", key)
		}
	}
}

func TestAccountDashboardFormatsSummaryNumbers(t *testing.T) {
	handler, router := testAccount(t)

	user, err := handler.store.CreateConsumerUser(context.Background(), "summary@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	user, err = handler.store.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, 5000000000)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1200; i++ {
		if err := handler.store.RecordRequest(context.Background(), store.RequestLog{
			Protocol:       "openai",
			Model:          "model-a",
			ConsumerUserID: &user.ID,
			ConsumerEmail:  user.Email,
			StatusCode:     http.StatusOK,
			LatencyMS:      1,
			InputTokens:    1000,
			OutputTokens:   1000,
		}); err != nil {
			t.Fatal(err)
		}
	}
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, user.ID, user.Email); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == sessionCookie || cookie.Name == csrfCookie {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, expected := range []string{"<strong>5B</strong>", "<strong>2.4M</strong>", "<strong>1.2K</strong>"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing formatted summary number %q in account dashboard:\n%s", expected, body)
		}
	}
}

func TestCompactNumber(t *testing.T) {
	tests := map[int64]string{
		-1:         "0",
		999:        "999",
		1200:       "1.2K",
		3400000:    "3.4M",
		5000000000: "5B",
	}
	for value, expected := range tests {
		if got := compactNumber(value); got != expected {
			t.Fatalf("compactNumber(%d) = %q, want %q", value, got, expected)
		}
	}
}

func accountCookies(rec *httptest.ResponseRecorder) ([]*http.Cookie, string) {
	var cookies []*http.Cookie
	var csrf string
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookie || cookie.Name == csrfCookie {
			cookies = append(cookies, cookie)
		}
		if cookie.Name == csrfCookie {
			csrf = cookie.Value
		}
	}
	return cookies, csrf
}

func testAccount(t *testing.T) (*Handler, chi.Router) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	handler := New(st)
	router := chi.NewRouter()
	handler.Register(router)
	return handler, router
}
