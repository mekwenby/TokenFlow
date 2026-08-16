package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"tokenflow/internal/auth"
	"tokenflow/internal/chat"
	"tokenflow/internal/store"
)

const (
	mobileTokenPrefix   = "tfm_"
	mobileSessionTTL    = 30 * 24 * time.Hour
	mobileRenewalWindow = 7 * 24 * time.Hour
	mobileBodyLimit     = 64 << 10
)

type Handler struct {
	store       *store.Store
	chatService *chat.Service
	now         func() time.Time
}

type mobileUser struct {
	ID                   int64  `json:"id"`
	Email                string `json:"email"`
	QuotaTotalTokens     int64  `json:"quota_total_tokens"`
	QuotaUsedTokens      int64  `json:"quota_used_tokens"`
	QuotaRemainingTokens int64  `json:"quota_remaining_tokens"`
}

type sessionResponse struct {
	AccessToken string     `json:"access_token,omitempty"`
	TokenType   string     `json:"token_type,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	User        mobileUser `json:"user"`
}

type requestSession struct {
	TokenHash string
	Session   store.MobileSession
}

type requestSessionKey struct{}

func New(st *store.Store, chatService *chat.Service) *Handler {
	return &Handler{store: st, chatService: chatService, now: time.Now}
}

func (h *Handler) SetClock(now func() time.Time) {
	if now == nil {
		h.now = time.Now
		return
	}
	h.now = now
}

func (h *Handler) Register(r chi.Router) {
	r.Post("/mobile/v1/session", h.createSession)
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Get("/mobile/v1/session", h.currentSession)
		r.Delete("/mobile/v1/session", h.deleteSession)
		chat.RegisterRoutes(r, chat.RouteConfig{
			BasePath:    "/mobile/v1/chat",
			Service:     h.chatService,
			Store:       h.store,
			Owner:       h.chatOwner,
			RequireCSRF: allowMobileWrite,
		})
	})
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}
	if !decodePayload(w, r, &payload) {
		return
	}
	payload.Email = strings.TrimSpace(payload.Email)
	if payload.Email == "" || payload.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "email and password are required")
		return
	}
	user, err := h.store.ConsumerUserByEmail(r.Context(), payload.Email)
	if err != nil || user.Status != store.ConsumerStatusEnabled || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	now := h.now().UTC()
	if err := h.store.DeleteExpiredMobileSessions(r.Context(), now); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	rawToken, err := auth.RandomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	token := mobileTokenPrefix + rawToken
	expiresAt := now.Add(mobileSessionTTL)
	if err := h.store.CreateMobileSession(r.Context(), store.MobileSession{
		ConsumerUserID: user.ID,
		TokenHash:      auth.HashKey(token),
		DeviceName:     truncateRunes(strings.TrimSpace(payload.DeviceName), 120),
		CreatedAt:      now,
		LastUsedAt:     now,
		ExpiresAt:      expiresAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	writeJSON(w, http.StatusCreated, sessionResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User:        userResponse(user),
	})
}

func (h *Handler) currentSession(w http.ResponseWriter, r *http.Request) {
	requestSession, ok := sessionFromRequest(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	user, err := h.store.ConsumerUser(r.Context(), requestSession.Session.ConsumerUserID)
	if err != nil || user.Status != store.ConsumerStatusEnabled {
		writeUnauthorized(w)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{ExpiresAt: requestSession.Session.ExpiresAt, User: userResponse(user)})
}

func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request) {
	requestSession, ok := sessionFromRequest(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	if err := h.store.DeleteMobileSessionByTokenHash(r.Context(), requestSession.TokenHash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.ExtractBearer(r.Header.Get("Authorization"))
		if !strings.HasPrefix(token, mobileTokenPrefix) {
			writeUnauthorized(w)
			return
		}
		tokenHash := auth.HashKey(token)
		session, err := h.store.MobileSessionByTokenHash(r.Context(), tokenHash)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		now := h.now().UTC()
		if !session.ExpiresAt.After(now) {
			_ = h.store.DeleteMobileSessionByTokenHash(r.Context(), tokenHash)
			writeUnauthorized(w)
			return
		}
		if session.UserStatus != store.ConsumerStatusEnabled {
			_ = h.store.RevokeMobileSessions(r.Context(), session.ConsumerUserID)
			writeUnauthorized(w)
			return
		}
		expiresAt := session.ExpiresAt
		if expiresAt.Sub(now) <= mobileRenewalWindow {
			expiresAt = now.Add(mobileSessionTTL)
		}
		if err := h.store.RenewMobileSession(r.Context(), tokenHash, now, expiresAt); err != nil {
			writeUnauthorized(w)
			return
		}
		session.LastUsedAt = now
		session.ExpiresAt = expiresAt
		ctx := context.WithValue(r.Context(), requestSessionKey{}, requestSession{TokenHash: tokenHash, Session: session})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) chatOwner(r *http.Request) (store.ChatOwner, bool) {
	session, ok := sessionFromRequest(r)
	if !ok {
		return store.ChatOwner{}, false
	}
	return store.ChatOwner{Type: store.ChatOwnerConsumer, ID: session.Session.ConsumerUserID, Name: session.Session.Email}, true
}

func allowMobileWrite(http.ResponseWriter, *http.Request) bool {
	return true
}

func sessionFromRequest(r *http.Request) (requestSession, bool) {
	session, ok := r.Context().Value(requestSessionKey{}).(requestSession)
	return session, ok
}

func userResponse(user store.ConsumerUser) mobileUser {
	return mobileUser{
		ID:                   user.ID,
		Email:                user.Email,
		QuotaTotalTokens:     user.QuotaTotalTokens,
		QuotaUsedTokens:      user.QuotaUsedTokens,
		QuotaRemainingTokens: user.QuotaRemainingTokens,
	}
}

func decodePayload(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, mobileBodyLimit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain exactly one JSON value")
		return false
	}
	return true
}

func writeUnauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthorized", "login required")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": message, "code": code, "retryable": false})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
