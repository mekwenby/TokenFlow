package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"tokenflow/internal/auth"
	"tokenflow/internal/chat"
	"tokenflow/internal/store"
)

func TestMobileSessionLoginStoresOnlyHashAndAuthenticates(t *testing.T) {
	handler, st, router := testMobile(t)
	user := createMobileUser(t, st, "mobile@example.com", "password123", 1000)

	rec := mobileRequest(router, http.MethodPost, "/mobile/v1/session", `{"email":"mobile@example.com","password":"wrong","device_name":"Pixel"}`, "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"code":"invalid_credentials"`) {
		t.Fatalf("invalid login response: %d %s", rec.Code, rec.Body.String())
	}

	token, response := mobileLogin(t, router, user.Email, "password123", "Pixel 9")
	if response.User.ID != user.ID || response.User.QuotaRemainingTokens != 1000 {
		t.Fatalf("unexpected login user: %#v", response.User)
	}
	if !strings.HasPrefix(token, mobileTokenPrefix) {
		t.Fatalf("unexpected token format: %q", token)
	}
	stored, err := st.MobileSessionByTokenHash(context.Background(), auth.HashKey(token))
	if err != nil || stored.ConsumerUserID != user.ID || stored.DeviceName != "Pixel 9" {
		t.Fatalf("stored session mismatch: %#v err=%v", stored, err)
	}
	if _, err := st.MobileSessionByTokenHash(context.Background(), token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("plain token must not be stored, got %v", err)
	}

	rec = mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), user.Email) {
		t.Fatalf("authenticated session response: %d %s", rec.Code, rec.Body.String())
	}
	rec = mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer should fail: %d %s", rec.Code, rec.Body.String())
	}

	_ = handler
}

func TestMobileSessionRollingExpiryAndCurrentDeviceLogout(t *testing.T) {
	handler, st, router := testMobile(t)
	user := createMobileUser(t, st, "devices@example.com", "password123", 1000)
	start := time.Date(2026, time.August, 15, 9, 0, 0, 0, time.UTC)
	handler.SetClock(func() time.Time { return start })
	first, _ := mobileLogin(t, router, user.Email, "password123", "Phone")
	second, _ := mobileLogin(t, router, user.Email, "password123", "Tablet")

	handler.SetClock(func() time.Time { return start.Add(24 * 24 * time.Hour) })
	rec := mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", first)
	if rec.Code != http.StatusOK {
		t.Fatalf("renew session response: %d %s", rec.Code, rec.Body.String())
	}
	renewed, err := st.MobileSessionByTokenHash(context.Background(), auth.HashKey(first))
	if err != nil {
		t.Fatal(err)
	}
	wantExpiry := start.Add(54 * 24 * time.Hour)
	if !renewed.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("rolling expiry=%s want=%s", renewed.ExpiresAt, wantExpiry)
	}

	rec = mobileRequest(router, http.MethodDelete, "/mobile/v1/session", "", first)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout response: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", first); rec.Code != http.StatusUnauthorized {
		t.Fatalf("logged out token still works: %d", rec.Code)
	}
	if rec := mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", second); rec.Code != http.StatusOK {
		t.Fatalf("other device was revoked: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMobileSessionExpiresAndDisabledUserIsRevoked(t *testing.T) {
	handler, st, router := testMobile(t)
	user := createMobileUser(t, st, "expire@example.com", "password123", 1000)
	start := time.Date(2026, time.August, 15, 9, 0, 0, 0, time.UTC)
	handler.SetClock(func() time.Time { return start })
	expiredToken, _ := mobileLogin(t, router, user.Email, "password123", "Old phone")
	handler.SetClock(func() time.Time { return start.Add(31 * 24 * time.Hour) })
	if rec := mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", expiredToken); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired token response: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := st.MobileSessionByTokenHash(context.Background(), auth.HashKey(expiredToken)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired token was not deleted: %v", err)
	}

	handler.SetClock(func() time.Time { return start })
	first, _ := mobileLogin(t, router, user.Email, "password123", "Phone")
	second, _ := mobileLogin(t, router, user.Email, "password123", "Tablet")
	if _, err := st.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusDisabled, user.QuotaTotalTokens); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{first, second} {
		if rec := mobileRequest(router, http.MethodGet, "/mobile/v1/session", "", token); rec.Code != http.StatusUnauthorized {
			t.Fatalf("disabled token response: %d %s", rec.Code, rec.Body.String())
		}
		if _, err := st.MobileSessionByTokenHash(context.Background(), auth.HashKey(token)); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("disabled account session was not revoked: %v", err)
		}
	}
}

func TestMobileChatUsesBearerWithoutCSRFAndScopesOwners(t *testing.T) {
	_, st, router := testMobile(t)
	firstUser := createMobileUser(t, st, "first@example.com", "password123", 1000)
	secondUser := createMobileUser(t, st, "second@example.com", "password123", 1000)
	if _, err := st.CreateProvider(context.Background(), store.ProviderInput{
		Name: "mobile-chat", Protocol: "openai", BaseAPI: "https://example.test/v1", APIKeyCipher: "cipher",
		DefaultModel: "model-a", Models: []string{"model-a"}, Enabled: true, IsDefault: true,
	}); err != nil {
		t.Fatal(err)
	}
	firstToken, _ := mobileLogin(t, router, firstUser.Email, "password123", "Phone")
	secondToken, _ := mobileLogin(t, router, secondUser.Email, "password123", "Phone")

	rec := mobileRequest(router, http.MethodPost, "/mobile/v1/chat/conversations", `{"title":"Mobile","model":"model-a","thinking_effort":"medium","max_tool_calls":7}`, firstToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("mobile chat write without csrf failed: %d %s", rec.Code, rec.Body.String())
	}
	var conversation store.ChatConversation
	if err := json.Unmarshal(rec.Body.Bytes(), &conversation); err != nil {
		t.Fatal(err)
	}
	if conversation.ConsumerUserID == nil || *conversation.ConsumerUserID != firstUser.ID {
		t.Fatalf("conversation owner mismatch: %#v", conversation)
	}

	path := "/mobile/v1/chat/conversations/" + strconv.FormatInt(conversation.ID, 10)
	if rec := mobileRequest(router, http.MethodGet, path, "", secondToken); rec.Code != http.StatusNotFound {
		t.Fatalf("second user could read first user's conversation: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mobileRequest(router, http.MethodPost, "/mobile/v1/chat/conversations", `{}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("chat without bearer should fail: %d %s", rec.Code, rec.Body.String())
	}
}

func testMobile(t *testing.T) (*Handler, *store.Store, http.Handler) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	handler := New(st, chat.NewService(st, nil, ""))
	router := chi.NewRouter()
	handler.Register(router)
	return handler, st, router
}

func createMobileUser(t *testing.T, st *store.Store, email, password string, quota int64) store.ConsumerUser {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.CreateConsumerUser(context.Background(), email, string(hash))
	if err != nil {
		t.Fatal(err)
	}
	user, err = st.UpdateConsumerUser(context.Background(), user.ID, store.ConsumerStatusEnabled, quota)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func mobileLogin(t *testing.T, router http.Handler, email, password, deviceName string) (string, sessionResponse) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"email": email, "password": password, "device_name": deviceName})
	if err != nil {
		t.Fatal(err)
	}
	rec := mobileRequest(router, http.MethodPost, "/mobile/v1/session", string(body), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	var response sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.AccessToken, response
}

func mobileRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	reader = strings.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
