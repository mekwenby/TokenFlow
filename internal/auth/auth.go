package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"tokenflow/internal/store"
)

const (
	SessionCookie = "gateway_session"
	CSRFCookie    = "gateway_csrf"
	SessionTTL    = 15 * 24 * time.Hour
	RenewalWindow = 7 * 24 * time.Hour
)

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	UserID        int64
	Username      string
	OwnerType     string
	CSRFToken     string
	CSRFTokenHash string
	ExpiresAt     time.Time
}

type Sessions struct {
	store         *store.Store
	ownerType     string
	ttl           time.Duration
	renewalWindow time.Duration
	sessionCookie string
	csrfCookie    string
	path          string
	now           func() time.Time
}

func NewSessions(st *store.Store) *Sessions {
	return NewScopedSessions(st, store.AuthSessionOwnerAdmin, SessionCookie, CSRFCookie, "/admin")
}

func NewScopedSessions(st *store.Store, ownerType, sessionCookie, csrfCookie, path string) *Sessions {
	return &Sessions{
		store:         st,
		ownerType:     ownerType,
		ttl:           SessionTTL,
		renewalWindow: RenewalWindow,
		sessionCookie: sessionCookie,
		csrfCookie:    csrfCookie,
		path:          path,
		now:           time.Now,
	}
}

func (s *Sessions) SetClock(now func() time.Time) {
	if now == nil {
		s.now = time.Now
		return
	}
	s.now = now
}

func (s *Sessions) Create(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, err := RandomToken(32)
	if err != nil {
		return err
	}
	csrf, err := RandomToken(24)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.store.DeleteExpiredAuthSessions(r.Context(), now); err != nil {
		return err
	}
	expires := now.Add(s.ttl)
	if err := s.store.CreateAuthSession(r.Context(), store.AuthSession{
		OwnerType:     s.ownerType,
		UserID:        userID,
		TokenHash:     HashKey(token),
		CSRFTokenHash: HashKey(csrf),
		CreatedAt:     now,
		ExpiresAt:     expires,
	}); err != nil {
		return err
	}
	s.setCookies(w, r, token, csrf, expires)
	return nil
}

func (s *Sessions) Authenticate(w http.ResponseWriter, r *http.Request) (Session, error) {
	cookie, err := r.Cookie(s.sessionCookie)
	if err != nil || cookie.Value == "" {
		if csrfCookie, csrfErr := r.Cookie(s.csrfCookie); csrfErr == nil && csrfCookie.Value != "" {
			s.ClearCookies(w, r)
		}
		return Session{}, ErrSessionNotFound
	}
	tokenHash := HashKey(cookie.Value)
	stored, err := s.store.AuthSessionByTokenHash(r.Context(), s.ownerType, tokenHash)
	if errors.Is(err, store.ErrNotFound) {
		s.ClearCookies(w, r)
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	now := s.now().UTC()
	if !stored.ExpiresAt.After(now) {
		if err := s.store.DeleteAuthSessionByTokenHash(r.Context(), s.ownerType, tokenHash); err != nil {
			return Session{}, err
		}
		s.ClearCookies(w, r)
		return Session{}, ErrSessionNotFound
	}
	if s.ownerType == store.AuthSessionOwnerConsumer && stored.UserStatus != store.ConsumerStatusEnabled {
		if err := s.store.RevokeAuthSessions(r.Context(), s.ownerType, stored.UserID); err != nil {
			return Session{}, err
		}
		s.ClearCookies(w, r)
		return Session{}, ErrSessionNotFound
	}

	csrfValue := ""
	if csrfCookie, csrfErr := r.Cookie(s.csrfCookie); csrfErr == nil {
		csrfValue = csrfCookie.Value
	}
	rotateCSRF := csrfValue == "" || subtle.ConstantTimeCompare([]byte(HashKey(csrfValue)), []byte(stored.CSRFTokenHash)) != 1
	renew := stored.ExpiresAt.Sub(now) <= s.renewalWindow
	if rotateCSRF {
		csrfValue, err = RandomToken(24)
		if err != nil {
			return Session{}, err
		}
	}
	if renew || rotateCSRF {
		expires := stored.ExpiresAt
		if renew {
			expires = now.Add(s.ttl)
		}
		csrfHash := HashKey(csrfValue)
		if err := s.store.RenewAuthSession(r.Context(), s.ownerType, tokenHash, csrfHash, now, expires); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				s.ClearCookies(w, r)
				return Session{}, ErrSessionNotFound
			}
			return Session{}, err
		}
		stored.ExpiresAt = expires
		stored.CSRFTokenHash = csrfHash
		s.setCookies(w, r, cookie.Value, csrfValue, expires)
	}

	return Session{
		UserID:        stored.UserID,
		Username:      stored.Username,
		OwnerType:     stored.OwnerType,
		CSRFToken:     csrfValue,
		CSRFTokenHash: stored.CSRFTokenHash,
		ExpiresAt:     stored.ExpiresAt,
	}, nil
}

func (s *Sessions) ValidateCSRF(r *http.Request) bool {
	session, ok := SessionFromRequest(r)
	if !ok || session.OwnerType != s.ownerType {
		return false
	}
	cookie, err := r.Cookie(s.csrfCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" {
		header = r.FormValue("csrf")
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) == 1 &&
		subtle.ConstantTimeCompare([]byte(HashKey(header)), []byte(session.CSRFTokenHash)) == 1
}

func (s *Sessions) RevokeAll(w http.ResponseWriter, r *http.Request) error {
	session, ok := SessionFromRequest(r)
	if !ok || session.OwnerType != s.ownerType {
		return ErrSessionNotFound
	}
	if err := s.store.RevokeAuthSessions(r.Context(), s.ownerType, session.UserID); err != nil {
		return err
	}
	s.ClearCookies(w, r)
	return nil
}

func (s *Sessions) ClearCookies(w http.ResponseWriter, r *http.Request) {
	expired := time.Unix(0, 0)
	secure := requestIsHTTPS(r)
	http.SetCookie(w, &http.Cookie{
		Name: s.sessionCookie, Value: "", Path: s.path, Expires: expired, MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: s.csrfCookie, Value: "", Path: s.path, Expires: expired, MaxAge: -1,
		Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Sessions) setCookies(w http.ResponseWriter, r *http.Request, token, csrf string, expires time.Time) {
	secure := requestIsHTTPS(r)
	remaining := expires.Sub(s.now().UTC())
	maxAge := int((remaining + time.Second - 1) / time.Second)
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.sessionCookie, Value: token, Path: s.path, Expires: expires,
		MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: s.csrfCookie, Value: csrf, Path: s.path, Expires: expires,
		MaxAge: maxAge, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func requestIsHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(proto, "https")
}

type sessionContextKey struct{}

func WithSession(r *http.Request, session Session) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, session))
}

func SessionFromRequest(r *http.Request) (Session, bool) {
	session, ok := r.Context().Value(sessionContextKey{}).(Session)
	return session, ok
}

func NewDistributionKey() (plain, prefix, hash string, err error) {
	token, err := RandomToken(32)
	if err != nil {
		return "", "", "", err
	}
	plain = "sk-" + token
	prefix = plain
	if len(prefix) > 14 {
		prefix = prefix[:14]
	}
	hash = HashKey(plain)
	return plain, prefix, hash, nil
}

func HashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func MatchKey(plain, expectedHash string) bool {
	actual := HashKey(plain)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expectedHash)) == 1
}

func ExtractBearer(header string) string {
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func RandomToken(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
