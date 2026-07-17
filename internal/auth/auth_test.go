package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tokenflow/internal/store"
)

func TestSessionsPersistAndRenewWithinWindow(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gateway.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	sessions := NewSessions(st)
	sessions.SetClock(func() time.Time { return base })
	createReq := httptest.NewRequest(http.MethodPost, "http://tokenflow.test/admin/login", nil)
	createReq.Header.Set("X-Forwarded-Proto", "https")
	createRec := httptest.NewRecorder()
	if err := sessions.Create(createRec, createReq, admin.ID); err != nil {
		t.Fatal(err)
	}
	cookies := sessionCookies(createRec, SessionCookie, CSRFCookie)
	if len(cookies) != 2 {
		t.Fatalf("expected two auth cookies, got %#v", createRec.Result().Cookies())
	}
	sessionCookie := cookies[SessionCookie]
	csrfCookie := cookies[CSRFCookie]
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || !csrfCookie.Secure {
		t.Fatalf("unexpected secure cookie flags: session=%#v csrf=%#v", sessionCookie, csrfCookie)
	}
	if csrfCookie.HttpOnly || sessionCookie.Domain != "" || csrfCookie.Domain != "" {
		t.Fatalf("unexpected cookie visibility or domain: session=%#v csrf=%#v", sessionCookie, csrfCookie)
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode || csrfCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected SameSite flags: session=%#v csrf=%#v", sessionCookie, csrfCookie)
	}
	if sessionCookie.MaxAge != int(SessionTTL/time.Second) || !sessionCookie.Expires.Equal(base.Add(SessionTTL)) {
		t.Fatalf("unexpected session lifetime: %#v", sessionCookie)
	}
	if _, err := st.AuthSessionByTokenHash(ctx, store.AuthSessionOwnerAdmin, sessionCookie.Value); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("raw session token must not be stored, got %v", err)
	}
	stored, err := st.AuthSessionByTokenHash(ctx, store.AuthSessionOwnerAdmin, HashKey(sessionCookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	if stored.CSRFTokenHash != HashKey(csrfCookie.Value) {
		t.Fatal("csrf token was not stored as the expected hash")
	}

	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sessions = NewSessions(st)

	sessions.SetClock(func() time.Time { return base.Add(7 * 24 * time.Hour) })
	nonRenewReq := requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies)
	nonRenewRec := httptest.NewRecorder()
	if _, err := sessions.Authenticate(nonRenewRec, nonRenewReq); err != nil {
		t.Fatal(err)
	}
	if got := nonRenewRec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("session with more than seven days remaining should not be renewed: %#v", got)
	}
	stored, err = st.AuthSessionByTokenHash(ctx, store.AuthSessionOwnerAdmin, HashKey(sessionCookie.Value))
	if err != nil || !stored.ExpiresAt.Equal(base.Add(SessionTTL)) {
		t.Fatalf("session outside the renewal window should remain unchanged: %#v err=%v", stored, err)
	}

	renewedAt := base.Add(8 * 24 * time.Hour)
	sessions.SetClock(func() time.Time { return renewedAt })
	renewReq := requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies)
	renewReq.Header.Set("X-Forwarded-Proto", "https")
	renewRec := httptest.NewRecorder()
	session, err := sessions.Authenticate(renewRec, renewReq)
	if err != nil {
		t.Fatal(err)
	}
	if session.Username != "admin" || !session.ExpiresAt.Equal(renewedAt.Add(SessionTTL)) {
		t.Fatalf("unexpected renewed session: %#v", session)
	}
	renewedCookies := sessionCookies(renewRec, SessionCookie, CSRFCookie)
	if len(renewedCookies) != 2 || !renewedCookies[SessionCookie].Expires.Equal(renewedAt.Add(SessionTTL)) {
		t.Fatalf("renewal did not refresh both cookies: %#v", renewRec.Result().Cookies())
	}

	csrfReq := requestWithCookies(http.MethodPost, "http://tokenflow.test/admin/action", renewedCookies)
	csrfReq.Header.Set("X-CSRF-Token", renewedCookies[CSRFCookie].Value)
	csrfReq = WithSession(csrfReq, session)
	if !sessions.ValidateCSRF(csrfReq) {
		t.Fatal("valid csrf token should be bound to the restored session")
	}
	csrfReq.Header.Set("X-CSRF-Token", "forged")
	if sessions.ValidateCSRF(csrfReq) {
		t.Fatal("forged csrf token should be rejected")
	}
}

func TestSessionsSupportMultipleDevicesAndLogoutRevokesAll(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, _ := st.AdminByUsername(ctx, "admin")
	consumer, err := st.CreateConsumerUser(ctx, "user@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	consumer, err = st.UpdateConsumerUser(ctx, consumer.ID, store.ConsumerStatusEnabled, 100)
	if err != nil {
		t.Fatal(err)
	}

	adminSessions := NewSessions(st)
	deviceOne := createSessionCookies(t, adminSessions, admin.ID, "/admin")
	deviceTwo := createSessionCookies(t, adminSessions, admin.ID, "/admin")
	consumerSessions := NewScopedSessions(st, store.AuthSessionOwnerConsumer, "consumer_session", "consumer_csrf", "/account")
	consumerCookies := createSessionCookies(t, consumerSessions, consumer.ID, "/account")

	logoutReq := requestWithCookies(http.MethodPost, "http://tokenflow.test/admin/logout", deviceOne)
	logoutReq.Header.Set("X-CSRF-Token", deviceOne[CSRFCookie].Value)
	logoutRec := httptest.NewRecorder()
	session, err := adminSessions.Authenticate(logoutRec, logoutReq)
	if err != nil {
		t.Fatal(err)
	}
	logoutReq = WithSession(logoutReq, session)
	if !adminSessions.ValidateCSRF(logoutReq) {
		t.Fatal("logout csrf should be valid")
	}
	if err := adminSessions.RevokeAll(logoutRec, logoutReq); err != nil {
		t.Fatal(err)
	}

	for index, cookies := range []map[string]*http.Cookie{deviceOne, deviceTwo} {
		req := requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies)
		rec := httptest.NewRecorder()
		if _, err := adminSessions.Authenticate(rec, req); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("admin device %d remained authenticated: %v", index+1, err)
		}
		if len(rec.Result().Cookies()) != 2 {
			t.Fatalf("revoked device should receive cookie deletion headers: %#v", rec.Result().Cookies())
		}
	}

	consumerReq := requestWithCookies(http.MethodGet, "http://tokenflow.test/account", consumerCookies)
	if _, err := consumerSessions.Authenticate(httptest.NewRecorder(), consumerReq); err != nil {
		t.Fatalf("admin logout must not revoke consumer with the same numeric id: %v", err)
	}
}

func TestExpiredSessionAndDatabaseErrorsAreDistinct(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, _ := st.AdminByUsername(ctx, "admin")
	base := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	sessions := NewSessions(st)
	sessions.SetClock(func() time.Time { return base })
	cookies := createSessionCookies(t, sessions, admin.ID, "/admin")

	sessions.SetClock(func() time.Time { return base.Add(SessionTTL + time.Second) })
	expiredRec := httptest.NewRecorder()
	if _, err := sessions.Authenticate(expiredRec, requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired session should be reported as missing, got %v", err)
	}
	if len(expiredRec.Result().Cookies()) != 2 {
		t.Fatalf("expired session should clear browser cookies: %#v", expiredRec.Result().Cookies())
	}

	sessions.SetClock(func() time.Time { return base })
	cookies = createSessionCookies(t, sessions, admin.ID, "/admin")
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	dbErrorRec := httptest.NewRecorder()
	_, err = sessions.Authenticate(dbErrorRec, requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies))
	if err == nil || errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("closed database should return an operational error, got %v", err)
	}
	if len(dbErrorRec.Result().Cookies()) != 0 {
		t.Fatalf("database errors must not clear cookies: %#v", dbErrorRec.Result().Cookies())
	}
}

func TestConcurrentRenewalCannotRestoreRevokedSession(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, _ := st.AdminByUsername(ctx, "admin")
	base := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	sessions := NewSessions(st)
	sessions.SetClock(func() time.Time { return base })
	cookies := createSessionCookies(t, sessions, admin.ID, "/admin")
	initialReq := requestWithCookies(http.MethodPost, "http://tokenflow.test/admin/logout", cookies)
	initialSession, err := sessions.Authenticate(httptest.NewRecorder(), initialReq)
	if err != nil {
		t.Fatal(err)
	}
	logoutReq := WithSession(initialReq, initialSession)

	sessions.SetClock(func() time.Time { return base.Add(8 * 24 * time.Hour) })
	start := make(chan struct{})
	errs := make(chan error, 9)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := sessions.Authenticate(httptest.NewRecorder(), requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies))
			if err != nil && !errors.Is(err, ErrSessionNotFound) {
				errs <- err
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		errs <- sessions.RevokeAll(httptest.NewRecorder(), logoutReq)
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sessions.Authenticate(httptest.NewRecorder(), requestWithCookies(http.MethodGet, "http://tokenflow.test/admin", cookies)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("renewal recreated a revoked session: %v", err)
	}
}

func TestRequestIsHTTPS(t *testing.T) {
	httpsReq := httptest.NewRequest(http.MethodGet, "https://tokenflow.test/admin", nil)
	forwardedReq := httptest.NewRequest(http.MethodGet, "http://tokenflow.test/admin", nil)
	forwardedReq.Header.Set("X-Forwarded-Proto", "https, http")
	httpReq := httptest.NewRequest(http.MethodGet, "http://tokenflow.test/admin", nil)
	if !requestIsHTTPS(httpsReq) || !requestIsHTTPS(forwardedReq) {
		t.Fatal("TLS and forwarded HTTPS requests should set Secure cookies")
	}
	if requestIsHTTPS(httpReq) {
		t.Fatal("plain local HTTP requests should remain usable without Secure cookies")
	}
}

func createSessionCookies(t *testing.T, sessions *Sessions, userID int64, path string) map[string]*http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://tokenflow.test"+path, nil)
	rec := httptest.NewRecorder()
	if err := sessions.Create(rec, req, userID); err != nil {
		t.Fatal(err)
	}
	return sessionCookies(rec, sessions.sessionCookie, sessions.csrfCookie)
}

func sessionCookies(rec *httptest.ResponseRecorder, names ...string) map[string]*http.Cookie {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	result := make(map[string]*http.Cookie, len(names))
	for _, cookie := range rec.Result().Cookies() {
		if wanted[cookie.Name] {
			result[cookie.Name] = cookie
		}
	}
	return result
}

func requestWithCookies(method, target string, cookies map[string]*http.Cookie) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req
}
