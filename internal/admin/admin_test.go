package admin

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

	"tokenflow/internal/auth"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
)

func TestLoginUsesAcceptLanguageAndCookieOverride(t *testing.T) {
	_, router := testAdmin(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="zh-CN">`) || !strings.Contains(body, "登录") {
		t.Fatalf("expected Chinese login page, got:\n%s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	req.Header.Set("Accept-Language", "zh-CN")
	req.AddCookie(&http.Cookie{Name: langCookie, Value: "en"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body = rec.Body.String()
	if !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, "Log in") {
		t.Fatalf("expected English login page, got:\n%s", body)
	}
}

func TestLanguagePostSetsCookieAndSanitizesNext(t *testing.T) {
	_, router := testAdmin(t)

	form := url.Values{"lang": {"zh-CN"}, "next": {"/admin/setup"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/lang", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/setup" {
		t.Fatalf("unexpected redirect: status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
	var found bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == langCookie && cookie.Value == "zh-CN" && cookie.Path == "/admin" {
			found = true
		}
	}
	if !found {
		t.Fatalf("language cookie was not set correctly: %#v", rec.Result().Cookies())
	}

	form = url.Values{"lang": {"en"}, "next": {"https://evil.example/admin"}}
	req = httptest.NewRequest(http.MethodPost, "/admin/lang", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin" {
		t.Fatalf("unsafe next was not sanitized: status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLanguagePostRejectsInvalidLanguage(t *testing.T) {
	_, router := testAdmin(t)
	form := url.Values{"lang": {"fr"}, "next": {"/admin"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/lang", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unsupported language") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestDashboardInjectsLanguageAndTranslations(t *testing.T) {
	handler, router := testAdmin(t)
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, 1, "admin"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: langCookie, Value: "zh-CN"})
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, expected := range []string{`<html lang="zh-CN">`, "TokenFlow", "tokenflow-logo.svg", "上游供应商", "支持模型", `id="api-addresses"`, `id="token-usage"`, `id="detail-modal"`, `id="logs-search-form"`, `window.__ADMIN_LANG__ = "zh-CN"`, `"requests":"请求数"`, `"api_addresses":"API 地址"`, `"token_usage":"Token 使用趋势"`, `"model_token_details":"模型 Token 明细"`, `"logs_search":"搜索请求"`, `"cache_hit_rate":"缓存命中率"`, `"previous_page":"上一页"`, `"next_page":"下一页"`, `"reset_key":"重新生成"`, `"reset_key_stats":"重置统计"`, `"distribution_key":"Key 名称"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in dashboard:\n%s", expected, body)
		}
	}
}

func TestKeysAPIIncludesCacheStatsAndResetsKey(t *testing.T) {
	handler, router := testAdmin(t)
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, 1, "admin"); err != nil {
		t.Fatal(err)
	}
	var cookies []*http.Cookie
	var csrf string
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			cookies = append(cookies, cookie)
		}
		if cookie.Name == auth.CSRFCookie {
			csrf = cookie.Value
		}
	}
	oldPlain := "sk-old-client"
	key, err := handler.store.CreateDistributionKey(context.Background(), "client", "sk-old", auth.HashKey(oldPlain))
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.store.UpdateKeyStats(context.Background(), key.ID, 1200, 300, 4500); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/keys", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", rec.Code, rec.Body.String())
	}
	var keys []store.DistributionKey
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].CacheReadTokens != 300 {
		t.Fatalf("cache stats were not returned: %#v", keys)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/keys/reset", strings.NewReader(`{"id":`+strconv.FormatInt(key.ID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reset without CSRF should be rejected, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/keys/reset", strings.NewReader(`{"id":`+strconv.FormatInt(key.ID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", rec.Code, rec.Body.String())
	}
	var reset store.DistributionKey
	if err := json.Unmarshal(rec.Body.Bytes(), &reset); err != nil {
		t.Fatal(err)
	}
	if reset.PlainKey == "" || reset.Prefix == "sk-old" || reset.RequestCount != 1 || reset.InputTokens != 1200 || reset.CacheReadTokens != 300 || reset.OutputTokens != 4500 {
		t.Fatalf("unexpected reset key response: %#v", reset)
	}
	if _, err := handler.store.DistributionKeyByHash(context.Background(), auth.HashKey(oldPlain)); err != store.ErrNotFound {
		t.Fatalf("old key should be invalid after reset, got %v", err)
	}
	if _, err := handler.store.DistributionKeyByHash(context.Background(), auth.HashKey(reset.PlainKey)); err != nil {
		t.Fatalf("new key hash should be valid: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/keys/reset-stats", strings.NewReader(`{"id":`+strconv.FormatInt(key.ID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reset stats without CSRF should be rejected, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/keys/reset-stats", strings.NewReader(`{"id":`+strconv.FormatInt(key.ID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected reset stats status: %d %s", rec.Code, rec.Body.String())
	}
	var cleared store.DistributionKey
	if err := json.Unmarshal(rec.Body.Bytes(), &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.PlainKey != "" || cleared.Prefix != reset.Prefix || cleared.RequestCount != 0 || cleared.InputTokens != 0 || cleared.CacheReadTokens != 0 || cleared.OutputTokens != 0 || cleared.LastUsedAt != nil {
		t.Fatalf("unexpected reset stats response: %#v", cleared)
	}
	if _, err := handler.store.DistributionKeyByHash(context.Background(), auth.HashKey(reset.PlainKey)); err != nil {
		t.Fatalf("key hash should remain valid after stats reset: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/keys/reset", strings.NewReader(`{"id":`+strconv.FormatInt(key.ID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated reset should be rejected, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestLogsAPIReturnsPaginationAndCacheFields(t *testing.T) {
	handler, router := testAdmin(t)
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, 1, "admin"); err != nil {
		t.Fatal(err)
	}
	key, err := handler.store.CreateDistributionKey(context.Background(), "client-key", "sk-client", "hash-client")
	if err != nil {
		t.Fatal(err)
	}
	keyID := key.ID
	if err := handler.store.InsertRequestLog(context.Background(), store.RequestLog{
		Protocol:    "openai",
		Model:       "older-model",
		StatusCode:  200,
		LatencyMS:   9,
		InputTokens: 10,
	}); err != nil {
		t.Fatal(err)
	}
	if err := handler.store.InsertRequestLog(context.Background(), store.RequestLog{
		Protocol:            "openai",
		Model:               "cached-model",
		StatusCode:          200,
		LatencyMS:           12,
		InputTokens:         20,
		OutputTokens:        4,
		CacheReadTokens:     5,
		CacheCreationTokens: 3,
		DistributionKeyID:   &keyID,
		DistributionKeyName: "client-key",
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/logs?limit=1&offset=0", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", rec.Code, rec.Body.String())
	}
	var page logsPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Limit != 1 || page.Offset != 0 || len(page.Items) != 1 {
		t.Fatalf("unexpected page metadata: %#v", page)
	}
	if page.Items[0].CacheReadTokens != 5 || page.Items[0].CacheCreationTokens != 3 || page.Items[0].CacheHitRate < 0.24 || page.Items[0].CacheHitRate > 0.26 {
		t.Fatalf("cache fields were not returned: %#v", page)
	}
	if page.Items[0].DistributionKeyID == nil || *page.Items[0].DistributionKeyID != key.ID || page.Items[0].DistributionKeyName != "client-key" {
		t.Fatalf("distribution key fields were not returned: %#v", page.Items[0])
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/logs?limit=-1&offset=-2", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Limit != 50 || page.Offset != 0 {
		t.Fatalf("invalid pagination params were not normalized: %#v", page)
	}
}

func TestLogsAPISearchesModelProviderAndKey(t *testing.T) {
	handler, router := testAdmin(t)
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, 1, "admin"); err != nil {
		t.Fatal(err)
	}
	provider, err := handler.store.CreateProvider(context.Background(), store.ProviderInput{
		Name:         "OpenAI Primary",
		Protocol:     "openai",
		BaseAPI:      "https://api.example.test/v1",
		APIKeyCipher: "cipher",
		DefaultModel: "gpt-4.1",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherProvider, err := handler.store.CreateProvider(context.Background(), store.ProviderInput{
		Name:         "Anthropic Backup",
		Protocol:     "anthropic",
		BaseAPI:      "https://anthropic.example",
		APIKeyCipher: "cipher",
		DefaultModel: "claude",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := handler.store.CreateDistributionKey(context.Background(), "Client A", "sk-a", "hash-a")
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := handler.store.CreateDistributionKey(context.Background(), "Client B", "sk-b", "hash-b")
	if err != nil {
		t.Fatal(err)
	}
	providerID, otherProviderID := provider.ID, otherProvider.ID
	keyID, otherKeyID := key.ID, otherKey.ID
	for _, log := range []store.RequestLog{
		{Protocol: "openai", Model: "gpt-4.1", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1},
		{Protocol: "openai", Model: "gpt-4.1-mini", ProviderID: &providerID, DistributionKeyID: &otherKeyID, StatusCode: 200, LatencyMS: 1},
		{Protocol: "anthropic", Model: "claude-3-5-sonnet", ProviderID: &otherProviderID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1},
	} {
		if err := handler.store.InsertRequestLog(context.Background(), log); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/logs?q="+url.QueryEscape("OpenAI Primary & Client B | claude"), nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", rec.Code, rec.Body.String())
	}
	var page logsPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Query != "OpenAI Primary & Client B | claude" || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected search page: %#v", page)
	}
	seen := map[string]bool{}
	for _, item := range page.Items {
		seen[item.Model] = true
	}
	if !seen["gpt-4.1-mini"] || !seen["claude-3-5-sonnet"] || seen["gpt-4.1"] {
		t.Fatalf("search returned wrong models: %#v", page.Items)
	}
}

func TestTokenUsageAPI(t *testing.T) {
	handler, router := testAdmin(t)
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, 1, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := handler.store.InsertRequestLog(context.Background(), store.RequestLog{
		Protocol:            "openai",
		Model:               "usage-model",
		StatusCode:          200,
		LatencyMS:           8,
		InputTokens:         12,
		OutputTokens:        5,
		CacheReadTokens:     4,
		CacheCreationTokens: 2,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/token-usage?tz_offset=480", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d %s", rec.Code, rec.Body.String())
	}
	var report store.TokenUsageReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Range != "24h" || report.Granularity != "hour" || report.TimezoneOffsetMinutes != 480 || len(report.Points) != 24 {
		t.Fatalf("unexpected default usage report: %#v", report)
	}
	requests, input, output, cacheRead, cacheCreate := sumUsagePoints(report.Points)
	if requests != 1 || input != 12 || output != 5 || cacheRead != 4 || cacheCreate != 2 {
		t.Fatalf("unexpected default usage totals: requests=%d input=%d output=%d cacheRead=%d cacheCreate=%d", requests, input, output, cacheRead, cacheCreate)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/token-usage?range=7d&tz_offset=480", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected 7d status: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Range != "7d" || report.Granularity != "day" || len(report.Points) != 7 {
		t.Fatalf("unexpected 7d usage report: %#v", report)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/token-usage?range=30d", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "range must be 24h or 7d") {
		t.Fatalf("invalid range was not rejected: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/token-usage", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated token usage should be rejected, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestModelTokenDetailsAPI(t *testing.T) {
	handler, router := testAdmin(t)
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, 1, "admin"); err != nil {
		t.Fatal(err)
	}
	provider, err := handler.store.CreateProvider(context.Background(), store.ProviderInput{
		Name:         "provider-a",
		Protocol:     "openai",
		BaseAPI:      "https://api.example.test/v1",
		APIKeyCipher: "cipher",
		DefaultModel: "fallback",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := handler.store.CreateDistributionKey(context.Background(), "client-a", "sk-client", "hash-client")
	if err != nil {
		t.Fatal(err)
	}
	providerID, keyID := provider.ID, key.ID
	for _, log := range []store.RequestLog{
		{Protocol: "openai", Model: "model-a", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 7, OutputTokens: 3, CacheReadTokens: 2, CacheCreationTokens: 1},
		{Protocol: "openai", Model: "model-a", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 8, OutputTokens: 4, CacheReadTokens: 3, CacheCreationTokens: 2},
		{Protocol: "openai", Model: "model-b", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 20, OutputTokens: 9, CacheReadTokens: 5, CacheCreationTokens: 4},
	} {
		if err := handler.store.InsertRequestLog(context.Background(), log); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/model-token-details?scope=provider&id="+strconv.FormatInt(provider.ID, 10), nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected provider status: %d %s", rec.Code, rec.Body.String())
	}
	var report store.ModelTokenDetailReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Scope != "provider" || report.ID != provider.ID || report.Name != "provider-a" || len(report.Items) != 2 {
		t.Fatalf("unexpected provider report: %#v", report)
	}
	if report.Items[0].Model != "model-b" || report.Items[0].TotalTokens != 29 || report.Items[1].Requests != 2 || report.Totals.InputTokens != 35 || report.Totals.CacheCreationTokens != 7 {
		t.Fatalf("unexpected provider token details: %#v", report)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/model-token-details?scope=key&id="+strconv.FormatInt(key.ID, 10), nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
			req.AddCookie(cookie)
		}
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected key status: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Scope != "key" || report.ID != key.ID || report.Name != "client-a" || report.Totals.Requests != 3 {
		t.Fatalf("unexpected key report: %#v", report)
	}

	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/admin/api/model-token-details?scope=bad&id=" + strconv.FormatInt(provider.ID, 10), http.StatusBadRequest, "scope must be provider or key"},
		{"/admin/api/model-token-details?scope=provider", http.StatusBadRequest, "id is required"},
		{"/admin/api/model-token-details?scope=provider&id=9999", http.StatusNotFound, "not found"},
	} {
		req = httptest.NewRequest(http.MethodGet, tc.path, nil)
		for _, cookie := range loginRecorder.Result().Cookies() {
			if cookie.Name == auth.SessionCookie || cookie.Name == auth.CSRFCookie {
				req.AddCookie(cookie)
			}
		}
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.body) {
			t.Fatalf("%s: got status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/model-token-details?scope=provider&id="+strconv.FormatInt(provider.ID, 10), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated details should be rejected, got %d %s", rec.Code, rec.Body.String())
	}
}

func sumUsagePoints(points []store.TokenUsagePoint) (requests, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64) {
	for _, point := range points {
		requests += point.Requests
		inputTokens += point.InputTokens
		outputTokens += point.OutputTokens
		cacheReadTokens += point.CacheReadTokens
		cacheCreationTokens += point.CacheCreationTokens
	}
	return requests, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens
}

func testAdmin(t *testing.T) (*Handler, chi.Router) {
	t.Helper()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.CreateAdmin(context.Background(), "admin", "$2a$10$uses.fake.hash.only.for.rendering.tests"); err != nil {
		t.Fatal(err)
	}
	handler := New(st, box)
	router := chi.NewRouter()
	handler.Register(router)
	return handler, router
}
