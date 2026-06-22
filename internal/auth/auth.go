package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	SessionCookie = "gateway_session"
	CSRFCookie    = "gateway_csrf"
)

type Session struct {
	UserID    int64
	Username  string
	ExpiresAt time.Time
}

type Sessions struct {
	mu            sync.Mutex
	sessions      map[string]Session
	ttl           time.Duration
	sessionCookie string
	csrfCookie    string
	path          string
}

func NewSessions(ttl time.Duration) *Sessions {
	return NewScopedSessions(ttl, SessionCookie, CSRFCookie, "/admin")
}

func NewScopedSessions(ttl time.Duration, sessionCookie, csrfCookie, path string) *Sessions {
	return &Sessions{
		sessions:      map[string]Session{},
		ttl:           ttl,
		sessionCookie: sessionCookie,
		csrfCookie:    csrfCookie,
		path:          path,
	}
}

func (s *Sessions) Create(w http.ResponseWriter, userID int64, username string) error {
	token, err := RandomToken(32)
	if err != nil {
		return err
	}
	csrf, err := RandomToken(24)
	if err != nil {
		return err
	}
	expires := time.Now().Add(s.ttl)
	s.mu.Lock()
	s.sessions[token] = Session{UserID: userID, Username: username, ExpiresAt: expires}
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookie,
		Value:    token,
		Path:     s.path,
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     s.csrfCookie,
		Value:    csrf,
		Path:     s.path,
		Expires:  expires,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Sessions) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}
	expired := time.Unix(0, 0)
	http.SetCookie(w, &http.Cookie{Name: s.sessionCookie, Value: "", Path: s.path, Expires: expired, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: s.csrfCookie, Value: "", Path: s.path, Expires: expired, MaxAge: -1})
}

func (s *Sessions) Get(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(s.sessionCookie)
	if err != nil || cookie.Value == "" {
		return Session{}, false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[cookie.Value]
	if !ok {
		return Session{}, false
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, cookie.Value)
		return Session{}, false
	}
	return session, true
}

func (s *Sessions) ValidateCSRF(r *http.Request) bool {
	return validateCSRF(r, s.csrfCookie)
}

func ValidateCSRF(r *http.Request) bool {
	return validateCSRF(r, CSRFCookie)
}

func validateCSRF(r *http.Request, cookieName string) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" {
		header = r.FormValue("csrf")
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) == 1
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
