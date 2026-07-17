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

	"tokenflow/internal/chat"
	"tokenflow/internal/secret"
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
	if err := handler.sessions.Create(otherRec, httptest.NewRequest(http.MethodGet, "/account", nil), other.ID); err != nil {
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

func TestAccountLogsAPIIsScopedAndSearchable(t *testing.T) {
	handler, router := testAccount(t)

	req := httptest.NewRequest(http.MethodGet, "/account/api/logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated logs should be rejected: %d %s", rec.Code, rec.Body.String())
	}

	user, err := handler.store.CreateConsumerUser(context.Background(), "viewer@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	user, err = handler.store.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, 1000)
	if err != nil {
		t.Fatal(err)
	}
	other, err := handler.store.CreateConsumerUser(context.Background(), "other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err = handler.store.UpdateConsumerUser(context.Background(), other.ID, store.ConsumerStatusEnabled, 1000)
	if err != nil {
		t.Fatal(err)
	}
	key, err := handler.store.CreateConsumerDistributionKey(context.Background(), user.ID, "Alpha % Key", "sk-alpha", "hash-alpha")
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := handler.store.CreateConsumerDistributionKey(context.Background(), other.ID, "Other Key", "sk-other", "hash-other")
	if err != nil {
		t.Fatal(err)
	}
	keyID, otherKeyID := key.ID, otherKey.ID
	userID, otherID := user.ID, other.ID
	for _, log := range []store.RequestLog{
		{Protocol: "openai", Model: "model-a", UpstreamModel: "hidden-upstream", DistributionKeyID: &keyID, DistributionKeyName: key.Name, ConsumerUserID: &userID, ConsumerEmail: user.Email, StatusCode: http.StatusOK, LatencyMS: 11, InputTokens: 10, CacheReadTokens: 4, CacheCreationTokens: 2, OutputTokens: 3, Stream: true},
		{Protocol: "anthropic", Model: "model-b", UpstreamModel: "hidden-upstream-b", DistributionKeyID: &keyID, DistributionKeyName: "Beta Key", ConsumerUserID: &userID, ConsumerEmail: user.Email, StatusCode: http.StatusBadGateway, LatencyMS: 22, InputTokens: 1, OutputTokens: 2},
		{Protocol: "openai", Model: "model-a", DistributionKeyID: &otherKeyID, DistributionKeyName: otherKey.Name, ConsumerUserID: &otherID, ConsumerEmail: other.Email, StatusCode: http.StatusOK, LatencyMS: 33},
	} {
		if err := handler.store.RecordRequest(context.Background(), log); err != nil {
			t.Fatal(err)
		}
	}
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, httptest.NewRequest(http.MethodGet, "/account", nil), user.ID); err != nil {
		t.Fatal(err)
	}
	cookies, _ := accountCookies(loginRecorder)

	req = httptest.NewRequest(http.MethodGet, "/account/api/logs?limit=999&offset=-2", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected logs status: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, hidden := range []string{"provider_name", "upstream_model", "consumer_email", "hidden-upstream", "other@example.com"} {
		if strings.Contains(body, hidden) {
			t.Fatalf("account logs response leaked %q: %s", hidden, body)
		}
	}
	var page accountLogsPage
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatal(err)
	}
	if page.Limit != 200 || page.Offset != 0 || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected logs page metadata: %#v", page)
	}
	if page.Items[0].Model != "model-b" || page.Items[0].StatusCode != http.StatusBadGateway || page.Items[1].DistributionKeyName != key.Name {
		t.Fatalf("unexpected logs order or fields: %#v", page.Items)
	}
	if page.Items[1].CacheReadTokens != 4 || page.Items[1].CacheCreationTokens != 2 || page.Items[1].CacheHitRate < 0.39 || page.Items[1].CacheHitRate > 0.41 || !page.Items[1].Stream {
		t.Fatalf("cache and stream fields were not returned: %#v", page.Items[1])
	}

	req = httptest.NewRequest(http.MethodGet, "/account/api/logs?q="+url.QueryEscape("Alpha % & model-a"), nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected filtered logs status: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].DistributionKeyName != key.Name {
		t.Fatalf("AND search with LIKE escaping returned unexpected page: %#v", page)
	}

	req = httptest.NewRequest(http.MethodGet, "/account/api/logs?q="+url.QueryEscape("Beta | Other"), nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected OR filtered logs status: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].DistributionKeyName != "Beta Key" {
		t.Fatalf("OR search should stay scoped to current user: %#v", page)
	}
}

func TestAccountChatAPIRequiresSessionCSRFAndQuota(t *testing.T) {
	handler, router := testAccount(t)
	if _, err := handler.store.CreateProvider(context.Background(), store.ProviderInput{Name: "chat-test", Protocol: "openai", BaseAPI: "https://example.test/v1", APIKeyCipher: "cipher", DefaultModel: "model-a", Models: []string{"model-a"}, Enabled: true, IsDefault: true}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/account/chat", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account/login" {
		t.Fatalf("unauthenticated chat page should redirect: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/account/api/chat/models", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated chat API should be rejected: %d %s", rec.Code, rec.Body.String())
	}

	user, err := handler.store.CreateConsumerUser(context.Background(), "chat-api@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	user, err = handler.store.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, 100)
	if err != nil {
		t.Fatal(err)
	}
	loginRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(loginRecorder, httptest.NewRequest(http.MethodGet, "/account", nil), user.ID); err != nil {
		t.Fatal(err)
	}
	cookies, csrf := accountCookies(loginRecorder)

	req = httptest.NewRequest(http.MethodGet, "/account/api/chat/models", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"models"`) || !strings.Contains(rec.Body.String(), `"default_system_prompt"`) || !strings.Contains(rec.Body.String(), `"max_tool_calls"`) || !strings.Contains(rec.Body.String(), `"max_user_message_chars":131072`) {
		t.Fatalf("unexpected chat models response: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/account/chat", nil)
	req.AddCookie(&http.Cookie{Name: adminLangCookie, Value: "zh-CN"})
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "一念通流 TokenFlow") {
		t.Fatalf("account chat page is missing the Chinese brand: %s", body)
	}
	assertAccountPWA(t, body)
	if !strings.Contains(body, `content="width=device-width, initial-scale=1, viewport-fit=cover"`) {
		t.Fatalf("account chat page should opt into safe-area viewport handling: %s", body)
	}
	for _, expected := range []string{`class="admin-page chat-page"`, `data-chat-root`, `data-chat-ready="false"`, `data-chat-lang=`, `data-chat-api-prefix="/account/api/chat"`, `data-chat-csrf-cookie="gateway_account_csrf"`, `data-chat-settings-writable="false"`, `data-chat-sidebar`, `data-chat-sidebar-toggle`, `data-chat-conversation-search`, `data-chat-account-menu`, `data-chat-tools-menu`, `data-chat-process`, `data-chat-scroll-bottom`, `data-chat-user-avatar`, `data-chat-assistant-avatar`, `data-chat-max-tool-calls`, `data-chat-default-system-prompt`, `data-chat-auto-title`, `css/chat.css`, `chat/app.js`, `href="/account"`} {
		if rec.Code != http.StatusOK || !strings.Contains(body, expected) {
			t.Fatalf("missing %q in account chat page: status=%d body=%s", expected, rec.Code, body)
		}
	}
	for _, unexpected := range []string{"account/app.js", `id="account-api-section"`, `class="topbar"`, `data-chat-process-shell`, `data-chat-character-count`} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("account chat page should not render %q:\n%s", unexpected, body)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/account/api/chat/conversations", strings.NewReader(`{"title":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("chat conversation create without CSRF should fail: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/account/api/chat/settings", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"max_tool_calls":6`) {
		t.Fatalf("unexpected chat settings response: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/account/api/chat/settings", strings.NewReader(`{"max_tool_calls":8}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("account chat settings patch should not be allowed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/api/chat/conversations", strings.NewReader(`{"title":"test","model":"model-a","thinking_effort":"low","user_avatar":"🧑‍🚀","assistant_avatar":"✨"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected chat conversation create status: %d %s", rec.Code, rec.Body.String())
	}
	var conv store.ChatConversation
	if err := json.Unmarshal(rec.Body.Bytes(), &conv); err != nil {
		t.Fatal(err)
	}
	if conv.ConsumerUserID == nil || *conv.ConsumerUserID != user.ID || conv.AdminUserID != nil || conv.UserAvatar != "🧑‍🚀" || conv.AssistantAvatar != "✨" {
		t.Fatalf("conversation was not scoped to account user: %#v", conv)
	}

	req = httptest.NewRequest(http.MethodPost, "/account/api/chat/conversations/"+strconv.FormatInt(conv.ID, 10)+"/title", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("chat title generation without CSRF should fail: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/api/chat/conversations/"+strconv.FormatInt(conv.ID, 10)+"/title", nil)
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no messages") {
		t.Fatalf("empty chat title generation should return 400: %d %s", rec.Code, rec.Body.String())
	}

	other, err := handler.store.CreateConsumerUser(context.Background(), "other-chat-api@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err = handler.store.UpdateConsumerUser(context.Background(), other.ID, store.ConsumerStatusEnabled, 100)
	if err != nil {
		t.Fatal(err)
	}
	otherRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(otherRecorder, httptest.NewRequest(http.MethodGet, "/account", nil), other.ID); err != nil {
		t.Fatal(err)
	}
	otherCookies, otherCSRF := accountCookies(otherRecorder)
	req = httptest.NewRequest(http.MethodGet, "/account/api/chat/conversations/"+strconv.FormatInt(conv.ID, 10), nil)
	for _, cookie := range otherCookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other account user should not read conversation: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/api/chat/conversations/"+strconv.FormatInt(conv.ID, 10)+"/title", nil)
	req.Header.Set("X-CSRF-Token", otherCSRF)
	for _, cookie := range otherCookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other account user should not generate title: %d %s", rec.Code, rec.Body.String())
	}

	limited, err := handler.store.CreateConsumerUser(context.Background(), "limited-chat-api@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	limited, err = handler.store.UpdateConsumerUser(context.Background(), limited.ID, store.ConsumerStatusEnabled, 0)
	if err != nil {
		t.Fatal(err)
	}
	limitedOwner := store.ChatOwner{Type: store.ChatOwnerConsumer, ID: limited.ID, Name: limited.Email}
	limitedConv, err := handler.store.CreateChatConversation(context.Background(), limitedOwner, "limited", "model-a", "medium", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	limitedRecorder := httptest.NewRecorder()
	if err := handler.sessions.Create(limitedRecorder, httptest.NewRequest(http.MethodGet, "/account", nil), limited.ID); err != nil {
		t.Fatal(err)
	}
	limitedCookies, limitedCSRF := accountCookies(limitedRecorder)
	req = httptest.NewRequest(http.MethodPost, "/account/api/chat/conversations/"+strconv.FormatInt(limitedConv.ID, 10)+"/messages", strings.NewReader(`{"content":"hello","model":"model-a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", limitedCSRF)
	for _, cookie := range limitedCookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("quota exceeded chat send should return 429 before streaming: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAccountPagesUseLanguageAndSharedScripts(t *testing.T) {
	handler, router := testAccount(t)

	req := httptest.NewRequest(http.MethodGet, "/account/register", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body := rec.Body.String()
	assertAccountPWA(t, body)
	if rec.Code != http.StatusOK || !strings.Contains(body, `<html lang="zh-CN">`) || !strings.Contains(body, "创建账号") || !strings.Contains(body, "注册") || !strings.Contains(body, "一念通流 TokenFlow") {
		t.Fatalf("expected Chinese register page, got status=%d body=%s", rec.Code, body)
	}

	req = httptest.NewRequest(http.MethodGet, "/account/login", nil)
	req.Header.Set("Accept-Language", "zh-CN")
	req.AddCookie(&http.Cookie{Name: adminLangCookie, Value: "en"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body = rec.Body.String()
	assertAccountPWA(t, body)
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
	if err := handler.sessions.Create(loginRecorder, httptest.NewRequest(http.MethodGet, "/account", nil), user.ID); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodGet, "/account", nil)
	req.AddCookie(&http.Cookie{Name: adminLangCookie, Value: "zh-CN"})
	csrfValue := ""
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == sessionCookie || cookie.Name == csrfCookie {
			req.AddCookie(cookie)
		}
		if cookie.Name == csrfCookie {
			csrfValue = cookie.Value
		}
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body = rec.Body.String()
	for _, expected := range []string{`<html lang="zh-CN">`, "使用情况", "API 地址", "创建 Key", `id="account-logs-search-form"`, `id="account-logs"`, `href="#account-logs-section"`, `window.__ACCOUNT_LANG__ = "zh-CN"`, `window.__ACCOUNT_I18N__`, `"recent_requests"`, `"copied":"已复制"`, "account/app.js"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in account dashboard:\n%s", expected, body)
		}
	}
	assertAccountPWA(t, body)
	if csrfValue == "" || !strings.Contains(body, `name="csrf" value="`+csrfValue+`"`) {
		t.Fatal("account logout form is missing its session-bound csrf token")
	}
	topbarStart := strings.Index(body, `<header class="topbar">`)
	if topbarStart < 0 {
		t.Fatal("account dashboard is missing top bar")
	}
	topbarEnd := strings.Index(body[topbarStart:], `</header>`)
	if topbarEnd < 0 {
		t.Fatal("account dashboard top bar is not closed")
	}
	topbar := body[topbarStart : topbarStart+topbarEnd]
	if strings.Contains(topbar, `href="/account/chat"`) {
		t.Fatalf("account top bar should not contain the moved chat entry:\n%s", topbar)
	}
	sideStart := strings.Index(body, `<aside class="side-nav"`)
	if sideStart < 0 {
		t.Fatal("account dashboard is missing desktop navigation")
	}
	sideEnd := strings.Index(body[sideStart:], `</aside>`)
	if sideEnd < 0 {
		t.Fatal("account dashboard desktop navigation is not closed")
	}
	sideNav := body[sideStart : sideStart+sideEnd]
	chatIndex := strings.Index(sideNav, `href="/account/chat"`)
	usageIndex := strings.Index(sideNav, `href="#account-usage-section"`)
	if chatIndex < 0 || usageIndex < 0 || chatIndex > usageIndex || !strings.Contains(sideNav, `icons.svg#icon-chat`) {
		t.Fatalf("account desktop navigation should start with the dedicated chat entry:\n%s", sideNav)
	}
	mobileStart := strings.Index(body, `<nav class="mobile-nav"`)
	if mobileStart < 0 {
		t.Fatal("account dashboard is missing mobile navigation")
	}
	mobileEnd := strings.Index(body[mobileStart:], `</nav>`)
	if mobileEnd < 0 {
		t.Fatal("account dashboard mobile navigation is not closed")
	}
	mobileNav := body[mobileStart : mobileStart+mobileEnd]
	chatIndex = strings.Index(mobileNav, `href="/account/chat"`)
	usageIndex = strings.Index(mobileNav, `href="#account-usage-section"`)
	if count := strings.Count(mobileNav, `<a class="nav-item`); count != 5 {
		t.Fatalf("account mobile navigation should have 5 entries, got %d", count)
	}
	if chatIndex < 0 || usageIndex < 0 || chatIndex > usageIndex {
		t.Fatalf("account mobile navigation should start with chat:\n%s", mobileNav)
	}
	for _, unexpected := range []string{`data-chat-root`, `account-chat-section`, `chat/app.js`} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("account dashboard should not include %q:\n%s", unexpected, body)
		}
	}
}

func assertAccountPWA(t *testing.T, body string) {
	t.Helper()
	for _, expected := range []string{`rel="manifest" href="/manifest.webmanifest"`, `name="theme-color" content="#101820"`, `src="/admin/static/pwa/register.js"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("account page is missing PWA declaration %q:\n%s", expected, body)
		}
	}
}

func TestAccountTranslationsIncludeChineseKeys(t *testing.T) {
	for key := range accountTranslations["en"] {
		if _, ok := accountTranslations["zh-CN"][key]; !ok {
			t.Fatalf("missing zh-CN account translation for %q", key)
		}
	}
	if got := accountTr("zh-CN", "app.title"); got != "一念通流 TokenFlow" {
		t.Fatalf("unexpected Chinese brand: %q", got)
	}
	if got := accountTr("en", "app.title"); got != "TokenFlow" {
		t.Fatalf("unexpected English brand: %q", got)
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
	if err := handler.sessions.Create(loginRecorder, httptest.NewRequest(http.MethodGet, "/account", nil), user.ID); err != nil {
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

func TestAccountLogoutRequiresCSRFAndRevokesAllDevices(t *testing.T) {
	handler, router := testAccount(t)
	user, err := handler.store.CreateConsumerUser(context.Background(), "logout@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	user, err = handler.store.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, 100)
	if err != nil {
		t.Fatal(err)
	}
	firstRec := httptest.NewRecorder()
	if err := handler.sessions.Create(firstRec, httptest.NewRequest(http.MethodGet, "/account", nil), user.ID); err != nil {
		t.Fatal(err)
	}
	firstCookies, firstCSRF := accountCookies(firstRec)
	secondRec := httptest.NewRecorder()
	if err := handler.sessions.Create(secondRec, httptest.NewRequest(http.MethodGet, "/account", nil), user.ID); err != nil {
		t.Fatal(err)
	}
	secondCookies, _ := accountCookies(secondRec)

	req := httptest.NewRequest(http.MethodPost, "/account/logout", nil)
	for _, cookie := range firstCookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logout without csrf should be rejected: %d %s", rec.Code, rec.Body.String())
	}

	form := url.Values{"csrf": {firstCSRF}}
	req = httptest.NewRequest(http.MethodPost, "/account/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range firstCookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account/login" {
		t.Fatalf("valid logout should redirect: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/account/api/keys", nil)
	for _, cookie := range secondCookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("second device should be revoked after logout: %d %s", rec.Code, rec.Body.String())
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
	handler := New(st)
	handler.SetChatService(chat.NewService(st, box, ""))
	router := chi.NewRouter()
	handler.Register(router)
	return handler, router
}
