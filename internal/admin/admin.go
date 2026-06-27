package admin

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"tokenflow/internal/auth"
	"tokenflow/internal/chat"
	"tokenflow/internal/httputil"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
	"tokenflow/web"
)

type Handler struct {
	store       *store.Store
	box         *secret.Box
	chatService *chat.Service
	sessions    *auth.Sessions
	tpl         *template.Template
}

type pageData struct {
	Title    string
	Username string
	Error    string
	Lang     string
	LangJSON template.JS
	I18NJSON template.JS
	Path     string
}

type providerPayload struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Protocol     string   `json:"protocol"`
	BaseAPI      string   `json:"base_api"`
	APIKey       string   `json:"api_key"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	Enabled      bool     `json:"enabled"`
	IsDefault    bool     `json:"is_default"`
}

type mappingPayload struct {
	ID            int64  `json:"id"`
	ClientModel   string `json:"client_model"`
	ProviderID    int64  `json:"provider_id"`
	UpstreamModel string `json:"upstream_model"`
}

type keyPayload struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type userPayload struct {
	ID               int64  `json:"id"`
	Status           string `json:"status"`
	QuotaTotalTokens int64  `json:"quota_total_tokens"`
}

type logsPage struct {
	Items  []store.RequestLog `json:"items"`
	Total  int64              `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
	Query  string             `json:"query"`
}

const (
	defaultLogsLimit = 50
	maxLogsLimit     = 200
)

func New(st *store.Store, box *secret.Box) *Handler {
	return &Handler{
		store:    st,
		box:      box,
		sessions: auth.NewSessions(12 * time.Hour),
		tpl: template.Must(template.New("admin").Funcs(template.FuncMap{
			"tr":         tr,
			"i18nJSON":   i18nJSON,
			"jsonString": jsonString,
		}).Parse(templates)),
	}
}

func (h *Handler) SetChatService(service *chat.Service) {
	h.chatService = service
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/admin/static/*", h.static)
	r.Get("/admin/setup", h.setupForm)
	r.Post("/admin/setup", h.setupPost)
	r.Get("/admin/login", h.loginForm)
	r.Post("/admin/login", h.loginPost)
	r.Post("/admin/logout", h.logout)
	r.Post("/admin/lang", h.languagePost)
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Get("/admin", h.dashboard)
		r.Get("/admin/chat", h.chatPage)
		r.Get("/admin/api/providers", h.providers)
		r.Post("/admin/api/providers", h.providers)
		r.Put("/admin/api/providers", h.providers)
		r.Delete("/admin/api/providers", h.providers)
		r.Get("/admin/api/model-mappings", h.mappings)
		r.Post("/admin/api/model-mappings", h.mappings)
		r.Put("/admin/api/model-mappings", h.mappings)
		r.Delete("/admin/api/model-mappings", h.mappings)
		r.Get("/admin/api/keys", h.keys)
		r.Post("/admin/api/keys", h.keys)
		r.Put("/admin/api/keys", h.keys)
		r.Delete("/admin/api/keys", h.keys)
		r.Post("/admin/api/keys/reset", h.resetKey)
		r.Post("/admin/api/keys/reset-stats", h.resetKeyStats)
		r.Get("/admin/api/users", h.users)
		r.Put("/admin/api/users", h.users)
		r.Get("/admin/api/stats", h.stats)
		r.Get("/admin/api/token-usage", h.tokenUsage)
		r.Get("/admin/api/model-token-details", h.modelTokenDetails)
		r.Get("/admin/api/logs", h.logs)
		chat.RegisterRoutes(r, chat.RouteConfig{
			BasePath:         "/admin/api/chat",
			Service:          h.chatService,
			Store:            h.store,
			Owner:            h.chatOwner,
			RequireCSRF:      h.requireCSRFForWrite,
			SettingsWritable: true,
		})
	})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessions.Get(r)
	data := h.page(r, "title.dashboard")
	data.Username = session.Username
	_ = h.tpl.ExecuteTemplate(w, "dashboard", data)
}

func (h *Handler) chatPage(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessions.Get(r)
	data := h.page(r, "title.dashboard")
	data.Title = "LLM Chat"
	data.Username = session.Username
	_ = h.tpl.ExecuteTemplate(w, "chat", data)
}

func (h *Handler) setupForm(w http.ResponseWriter, r *http.Request) {
	if h.hasAdmin(r) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	_ = h.tpl.ExecuteTemplate(w, "setup", h.page(r, "title.setup"))
}

func (h *Handler) setupPost(w http.ResponseWriter, r *http.Request) {
	if h.hasAdmin(r) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || len(password) < 8 {
		data := h.page(r, "title.setup")
		data.Error = tr(data.Lang, "setup_password_error")
		_ = h.tpl.ExecuteTemplate(w, "setup", data)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.CreateAdmin(r.Context(), username, string(hash)); err != nil {
		data := h.page(r, "title.setup")
		data.Error = err.Error()
		_ = h.tpl.ExecuteTemplate(w, "setup", data)
		return
	}
	user, _ := h.store.AdminByUsername(r.Context(), username)
	_ = h.sessions.Create(w, user.ID, user.Username)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handler) loginForm(w http.ResponseWriter, r *http.Request) {
	if !h.hasAdmin(r) {
		http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
		return
	}
	_ = h.tpl.ExecuteTemplate(w, "login", h.page(r, "title.login"))
}

func (h *Handler) loginPost(w http.ResponseWriter, r *http.Request) {
	if !h.hasAdmin(r) {
		http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	user, err := h.store.AdminByUsername(r.Context(), username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		data := h.page(r, "title.login")
		data.Error = tr(data.Lang, "login_invalid")
		_ = h.tpl.ExecuteTemplate(w, "login", data)
		return
	}
	if err := h.sessions.Create(w, user.ID, user.Username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w, r)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (h *Handler) languagePost(w http.ResponseWriter, r *http.Request) {
	lang, ok := normalizeLang(r.FormValue("lang"))
	if !ok {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "unsupported_language")
		return
	}
	setLanguageCookie(w, lang)
	http.Redirect(w, r, safeAdminNext(r.FormValue("next")), http.StatusSeeOther)
}

func (h *Handler) providers(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		providers, err := h.store.Providers(r.Context())
		h.writeResult(w, r, providers, err)
	case http.MethodPost:
		var payload providerPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		input, ok := h.providerInput(w, r, payload, true)
		if !ok {
			return
		}
		provider, err := h.store.CreateProvider(r.Context(), input)
		h.writeResult(w, r, sanitizeProvider(provider), err)
	case http.MethodPut:
		var payload providerPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		input, ok := h.providerInput(w, r, payload, false)
		if !ok {
			return
		}
		provider, err := h.store.UpdateProvider(r.Context(), payload.ID, input)
		h.writeResult(w, r, sanitizeProvider(provider), err)
	case http.MethodDelete:
		id := idParam(r)
		h.writeResult(w, r, map[string]bool{"ok": true}, h.store.DeleteProvider(r.Context(), id))
	}
}

func (h *Handler) providerInput(w http.ResponseWriter, r *http.Request, payload providerPayload, requireKey bool) (store.ProviderInput, bool) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Protocol = strings.TrimSpace(payload.Protocol)
	payload.BaseAPI = strings.TrimRight(strings.TrimSpace(payload.BaseAPI), "/")
	payload.DefaultModel = strings.TrimSpace(payload.DefaultModel)
	if payload.Name == "" || payload.BaseAPI == "" || payload.DefaultModel == "" {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "provider_required")
		return store.ProviderInput{}, false
	}
	if payload.Protocol != "openai" && payload.Protocol != "anthropic" {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "protocol_required")
		return store.ProviderInput{}, false
	}
	if requireKey && payload.APIKey == "" {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "api_key_required")
		return store.ProviderInput{}, false
	}
	var cipher string
	if payload.APIKey != "" {
		encrypted, err := h.box.Encrypt(payload.APIKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return store.ProviderInput{}, false
		}
		cipher = encrypted
	}
	return store.ProviderInput{
		Name:         payload.Name,
		Protocol:     payload.Protocol,
		BaseAPI:      payload.BaseAPI,
		APIKeyCipher: cipher,
		DefaultModel: payload.DefaultModel,
		Models:       payload.Models,
		Enabled:      payload.Enabled,
		IsDefault:    payload.IsDefault,
	}, true
}

func (h *Handler) mappings(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		mappings, err := h.store.Mappings(r.Context())
		h.writeResult(w, r, mappings, err)
	case http.MethodPost:
		var payload mappingPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		mapping, err := h.store.CreateMapping(r.Context(), strings.TrimSpace(payload.ClientModel), payload.ProviderID, strings.TrimSpace(payload.UpstreamModel))
		h.writeResult(w, r, mapping, err)
	case http.MethodPut:
		var payload mappingPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		mapping, err := h.store.UpdateMapping(r.Context(), payload.ID, strings.TrimSpace(payload.ClientModel), payload.ProviderID, strings.TrimSpace(payload.UpstreamModel))
		h.writeResult(w, r, mapping, err)
	case http.MethodDelete:
		id := idParam(r)
		h.writeResult(w, r, map[string]bool{"ok": true}, h.store.DeleteMapping(r.Context(), id))
	}
}

func (h *Handler) keys(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		keys, err := h.store.DistributionKeys(r.Context())
		h.writeResult(w, r, keys, err)
	case http.MethodPost:
		var payload keyPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		plain, prefix, hash, err := auth.NewDistributionKey()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		key, err := h.store.CreateDistributionKey(r.Context(), strings.TrimSpace(payload.Name), prefix, hash)
		key.PlainKey = plain
		h.writeResult(w, r, key, err)
	case http.MethodPut:
		var payload keyPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		key, err := h.store.UpdateDistributionKey(r.Context(), payload.ID, strings.TrimSpace(payload.Name), payload.Enabled)
		h.writeResult(w, r, key, err)
	case http.MethodDelete:
		id := idParam(r)
		h.writeResult(w, r, map[string]bool{"ok": true}, h.store.DeleteDistributionKey(r.Context(), id))
	}
}

func (h *Handler) resetKey(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	var payload keyPayload
	if !h.decodePayload(w, r, &payload) {
		return
	}
	plain, prefix, hash, err := auth.NewDistributionKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	key, err := h.store.ResetDistributionKey(r.Context(), payload.ID, prefix, hash)
	key.PlainKey = plain
	h.writeResult(w, r, key, err)
}

func (h *Handler) resetKeyStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	var payload keyPayload
	if !h.decodePayload(w, r, &payload) {
		return
	}
	key, err := h.store.ResetDistributionKeyStats(r.Context(), payload.ID)
	h.writeResult(w, r, key, err)
}

func (h *Handler) users(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		users, err := h.store.ConsumerUsers(r.Context())
		h.writeResult(w, r, users, err)
	case http.MethodPut:
		var payload userPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		user, err := h.store.UpdateConsumerUser(r.Context(), payload.ID, strings.TrimSpace(payload.Status), payload.QuotaTotalTokens)
		if errors.Is(err, store.ErrInvalidUserStatus) {
			h.writeLocalizedError(w, r, http.StatusBadRequest, "invalid_user_status")
			return
		}
		h.writeResult(w, r, user, err)
	}
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	h.writeResult(w, r, stats, err)
}

func (h *Handler) tokenUsage(w http.ResponseWriter, r *http.Request) {
	tzOffset, _ := strconv.Atoi(r.URL.Query().Get("tz_offset"))
	report, err := h.store.TokenUsage(r.Context(), r.URL.Query().Get("range"), tzOffset)
	if errors.Is(err, store.ErrInvalidUsageRange) {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "invalid_usage_range")
		return
	}
	h.writeResult(w, r, report, err)
}

func (h *Handler) modelTokenDetails(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if id <= 0 {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "detail_id_required")
		return
	}
	report, err := h.store.ModelTokenDetails(r.Context(), r.URL.Query().Get("scope"), id)
	if errors.Is(err, store.ErrInvalidModelScope) {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "invalid_detail_scope")
		return
	}
	h.writeResult(w, r, report, err)
}

func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, offset = normalizeLogsParams(limit, offset)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	logs, err := h.store.LogsSearch(r.Context(), limit, offset, query)
	if err != nil {
		h.writeResult(w, r, nil, err)
		return
	}
	total, err := h.store.LogCountSearch(r.Context(), query)
	h.writeResult(w, r, logsPage{Items: logs, Total: total, Limit: limit, Offset: offset, Query: query}, err)
}

func (h *Handler) chatOwner(r *http.Request) (store.ChatOwner, bool) {
	session, ok := h.sessions.Get(r)
	if !ok {
		return store.ChatOwner{}, false
	}
	return store.ChatOwner{Type: store.ChatOwnerAdmin, ID: session.UserID, Name: session.Username}, true
}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.hasAdmin(r) {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}
		if _, ok := h.sessions.Get(r); !ok {
			if strings.HasPrefix(r.URL.Path, "/admin/api/") {
				h.writeLocalizedError(w, r, http.StatusUnauthorized, "login_required")
			} else {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requireCSRFForWrite(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	if auth.ValidateCSRF(r) {
		return true
	}
	h.writeLocalizedError(w, r, http.StatusForbidden, "csrf_invalid")
	return false
}

func (h *Handler) hasAdmin(r *http.Request) bool {
	has, err := h.store.HasAdmin(r.Context())
	return err == nil && has
}

func (h *Handler) static(w http.ResponseWriter, r *http.Request) {
	http.StripPrefix("/admin/", http.FileServer(http.FS(web.Static))).ServeHTTP(w, r)
}

func sanitizeProvider(provider store.Provider) store.Provider {
	provider.APIKeyCipher = ""
	provider.PlainAPIKey = ""
	provider.HasAPIKey = true
	return provider
}

func (h *Handler) page(r *http.Request, titleKey string) pageData {
	lang := languageFromRequest(r)
	return pageData{
		Title:    tr(lang, titleKey),
		Lang:     lang,
		LangJSON: jsonString(lang),
		I18NJSON: i18nJSON(lang),
		Path:     safeAdminNext(r.URL.RequestURI()),
	}
}

func (h *Handler) decodePayload(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		h.writeLocalizedError(w, r, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func (h *Handler) writeResult(w http.ResponseWriter, r *http.Request, body any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, tr(languageFromRequest(r), "not_found"))
			return
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) writeLocalizedError(w http.ResponseWriter, r *http.Request, status int, key string) {
	httputil.WriteError(w, status, tr(languageFromRequest(r), key))
}

func writeError(w http.ResponseWriter, status int, message string) {
	httputil.WriteError(w, status, message)
}

func idParam(r *http.Request) int64 {
	return httputil.IDParam(r)
}

func normalizeLogsParams(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLogsLimit
	}
	if limit > maxLogsLimit {
		limit = maxLogsLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

const templates = `
{{define "lang_switch"}}
<form class="lang-switch" method="post" action="/admin/lang">
  <input type="hidden" name="next" value="{{.Path}}">
  <label>
    <span>{{tr .Lang "language"}}</span>
    <select name="lang" onchange="this.form.submit()">
      <option value="zh-CN" {{if eq .Lang "zh-CN"}}selected{{end}}>{{tr .Lang "lang.zh"}}</option>
      <option value="en" {{if eq .Lang "en"}}selected{{end}}>{{tr .Lang "lang.en"}}</option>
    </select>
  </label>
</form>
{{end}}

{{define "login"}}
<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" href="/admin/static/tokenflow-logo.svg"><link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/charts.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script></head>
<body class="auth-page">
  <main class="auth-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      {{template "lang_switch" .}}
    </div>
    <form method="post" action="/admin/login">
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <label>{{tr .Lang "username"}}<input name="username" autocomplete="username" required></label>
      <label>{{tr .Lang "password"}}<input name="password" type="password" autocomplete="current-password" required></label>
      <button type="submit">{{tr .Lang "login"}}</button>
    </form>
  </main>
</body>
</html>
{{end}}

{{define "setup"}}
<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" href="/admin/static/tokenflow-logo.svg"><link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/charts.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script></head>
<body class="auth-page">
  <main class="auth-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      {{template "lang_switch" .}}
    </div>
    <h2 class="auth-title">{{tr .Lang "initial_setup"}}</h2>
    <form method="post" action="/admin/setup">
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <label>{{tr .Lang "username"}}<input name="username" autocomplete="username" required></label>
      <label>{{tr .Lang "password"}}<input name="password" type="password" autocomplete="new-password" minlength="8" required></label>
      <button type="submit">{{tr .Lang "create_admin"}}</button>
    </form>
  </main>
</body>
</html>
{{end}}

{{define "dashboard"}}
<!doctype html>
<html lang="{{.Lang}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="icon" href="/admin/static/tokenflow-logo.svg">
  <link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/charts.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script>
</head>
<body class="admin-page">
  <header class="topbar">
    <div class="topbar-inner">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      <div class="top-actions">
        <a class="button-link secondary icon-label" href="/admin/chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>LLM Chat</span></a>
        {{template "lang_switch" .}}
        <span class="user-chip" title="{{.Username}}">{{.Username}}</span>
        <form method="post" action="/admin/logout">
          <button type="submit" class="secondary icon-label">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg>
            <span>{{tr .Lang "logout"}}</span>
          </button>
        </form>
      </div>
    </div>
  </header>
  <div class="app-shell">
    <aside class="side-nav" aria-label="Admin sections">
      <a class="nav-item active" href="#overview" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "overview"}}</span></a>
      <a class="nav-item" href="#api-addresses-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>{{tr .Lang "api_addresses"}}</span></a>
      <a class="nav-item" href="#users-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-users"></use></svg><span>{{tr .Lang "users"}}</span></a>
      <a class="nav-item" href="#providers-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-server"></use></svg><span>{{tr .Lang "providers"}}</span></a>
      <a class="nav-item" href="#mappings-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg><span>{{tr .Lang "model_mappings"}}</span></a>
      <a class="nav-item" href="#keys-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg><span>{{tr .Lang "distribution_keys"}}</span></a>
      <a class="nav-item" href="#logs-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>{{tr .Lang "recent_requests"}}</span></a>
    </aside>
  <main class="layout">
    <section id="overview" class="panel stats-panel">
      <h2>{{tr .Lang "overview"}}</h2>
      <div id="stats" class="stats-grid"></div>
      <div class="usage-section">
        <div class="section-head usage-head">
          <h3>{{tr .Lang "token_usage"}}</h3>
          <div class="segmented" role="group" aria-label="{{tr .Lang "usage_range"}}">
            <button type="button" data-action="usage-range" data-id="24h">{{tr .Lang "last_24_hours"}}</button>
            <button type="button" data-action="usage-range" data-id="7d">{{tr .Lang "last_7_days"}}</button>
          </div>
        </div>
        <div id="token-usage" class="usage-chart"></div>
      </div>
    </section>
    <section id="api-addresses-section" class="panel">
      <h2>{{tr .Lang "api_addresses"}}</h2>
      <div id="api-addresses" class="api-grid"></div>
    </section>
    <section id="users-section" class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "users"}}</h2>
      </div>
      <form id="user-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "status"}}<select name="status"><option value="pending">{{tr .Lang "pending"}}</option><option value="enabled">{{tr .Lang "enabled"}}</option><option value="disabled">{{tr .Lang "disabled"}}</option></select></label>
        <label>{{tr .Lang "quota_total_tokens"}}<input name="quota_total_tokens" type="number" min="0" step="1" required></label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-action="cancel">{{tr .Lang "cancel"}}</button></div>
      </form>
      <div id="users" class="table-wrap"></div>
    </section>
    <section id="providers-section" class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "providers"}}</h2>
        <button type="button" class="icon-label" data-action="open" data-id="provider-form">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
          <span>{{tr .Lang "add_provider"}}</span>
        </button>
      </div>
      <form id="provider-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "name"}}<input name="name" required></label>
        <label>{{tr .Lang "protocol"}}<select name="protocol"><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option></select></label>
        <label>{{tr .Lang "base_api"}}<input name="base_api" placeholder="https://api.openai.com/v1" required></label>
        <label>{{tr .Lang "api_key"}}<input name="api_key" type="password" placeholder="{{tr .Lang "api_key_placeholder"}}"></label>
        <label>{{tr .Lang "default_model"}}<input name="default_model" required></label>
        <label class="wide">{{tr .Lang "supported_models"}}<textarea name="models" rows="4" placeholder="{{tr .Lang "models_placeholder"}}"></textarea></label>
        <label class="check"><input name="enabled" type="checkbox" checked> {{tr .Lang "enabled"}}</label>
        <label class="check"><input name="is_default" type="checkbox"> {{tr .Lang "default"}}</label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-action="cancel">{{tr .Lang "cancel"}}</button></div>
      </form>
      <div id="providers" class="table-wrap"></div>
    </section>
    <section id="mappings-section" class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "model_mappings"}}</h2>
        <button type="button" class="icon-label" data-action="open" data-id="mapping-form">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
          <span>{{tr .Lang "add_mapping"}}</span>
        </button>
      </div>
      <form id="mapping-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "client_model"}}<input name="client_model" required></label>
        <label>{{tr .Lang "provider"}}<select name="provider_id" required></select></label>
        <label>{{tr .Lang "upstream_model"}}<input name="upstream_model" required></label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-action="cancel">{{tr .Lang "cancel"}}</button></div>
      </form>
      <div id="mappings" class="table-wrap"></div>
    </section>
    <section id="keys-section" class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "distribution_keys"}}</h2>
        <button type="button" class="icon-label" data-action="open" data-id="key-form">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
          <span>{{tr .Lang "create_key"}}</span>
        </button>
      </div>
      <form id="key-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "name"}}<input name="name" required></label>
        <label class="check key-enabled hidden"><input name="enabled" type="checkbox" checked> {{tr .Lang "enabled"}}</label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-action="cancel">{{tr .Lang "cancel"}}</button></div>
      </form>
      <p id="new-key" class="secret hidden"></p>
      <div id="keys" class="table-wrap"></div>
    </section>
    <section id="logs-section" class="panel">
      <div class="section-head logs-head">
        <h2>{{tr .Lang "recent_requests"}}</h2>
        <form id="logs-search-form" class="logs-search">
          <input name="q" type="search" placeholder="{{tr .Lang "logs_search_placeholder"}}" aria-label="{{tr .Lang "logs_search"}}">
          <button type="submit" class="action-icon" title="{{tr .Lang "logs_search"}}" aria-label="{{tr .Lang "logs_search"}}">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg>
          </button>
          <button type="button" class="secondary action-icon" data-action="clear-logs-search" title="{{tr .Lang "clear"}}" aria-label="{{tr .Lang "clear"}}">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg>
          </button>
        </form>
      </div>
      <div id="logs" class="table-wrap"></div>
      <div id="logs-pager"></div>
    </section>
  </main>
  </div>
  <nav class="mobile-nav" aria-label="Admin sections">
    <a class="nav-item active" href="#overview" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "overview"}}</span></a>
    <a class="nav-item" href="#api-addresses-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>{{tr .Lang "api_addresses"}}</span></a>
    <a class="nav-item" href="#users-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-users"></use></svg><span>{{tr .Lang "users"}}</span></a>
    <a class="nav-item" href="#providers-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-server"></use></svg><span>{{tr .Lang "providers"}}</span></a>
    <a class="nav-item" href="#mappings-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg><span>{{tr .Lang "model_mappings"}}</span></a>
    <a class="nav-item" href="#keys-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg><span>{{tr .Lang "distribution_keys"}}</span></a>
    <a class="nav-item" href="#logs-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>{{tr .Lang "recent_requests"}}</span></a>
  </nav>
  <div id="detail-modal" class="modal hidden" role="dialog" aria-modal="true" aria-labelledby="detail-modal-title">
    <div class="modal-dialog">
      <div class="modal-head">
        <div>
          <h2 id="detail-modal-title">{{tr .Lang "model_token_details"}}</h2>
          <p id="detail-modal-subtitle"></p>
        </div>
        <button type="button" class="secondary icon-label" data-action="close-detail">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg>
          <span>{{tr .Lang "close"}}</span>
        </button>
      </div>
      <div id="detail-modal-body"></div>
    </div>
  </div>
  <script>
    window.__ADMIN_LANG__ = {{.LangJSON}};
    window.__ADMIN_I18N__ = {{.I18NJSON}};
  </script>
  <script type="module" src="/admin/static/admin/app.js"></script>
</body>
</html>
{{end}}

{{define "chat"}}
<!doctype html>
<html lang="{{.Lang}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="icon" href="/admin/static/tokenflow-logo.svg">
  <link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script>
</head>
<body class="admin-page chat-page">
  <header class="topbar">
    <div class="topbar-inner">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>LLM Chat</span></h1>
      <div class="top-actions">
        <a class="button-link secondary icon-label" href="/admin"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "overview"}}</span></a>
        {{template "lang_switch" .}}
        <span class="user-chip" title="{{.Username}}">{{.Username}}</span>
        <form method="post" action="/admin/logout">
          <button type="submit" class="secondary icon-label">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg>
            <span>{{tr .Lang "logout"}}</span>
          </button>
        </form>
      </div>
    </div>
  </header>
  <main class="chat-app" data-chat-root data-chat-lang="{{.Lang}}" data-chat-api-prefix="/admin/api/chat" data-chat-csrf-cookie="gateway_csrf" data-chat-settings-writable="true">
    <aside class="chat-app-sidebar" aria-label="Conversations">
      <div class="chat-sidebar-head">
        <div>
          <strong>TokenFlow</strong>
          <span>LLM Chat</span>
        </div>
        <button type="button" class="action-icon" data-chat-new title="New chat" aria-label="New chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg></button>
      </div>
      <div class="chat-list" data-chat-conversations></div>
      <div class="chat-sidebar-foot">
        <button type="button" class="chat-settings-button" data-chat-settings-open title="Chat settings" aria-label="Chat settings"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-settings"></use></svg><span>Settings</span></button>
      </div>
    </aside>
    <section class="chat-app-main" aria-label="LLM Chat">
      <div class="chat-app-toolbar">
        <button type="button" class="chat-settings-summary" data-chat-settings-open title="Chat settings" aria-label="Chat settings">
          <span>Settings</span>
          <strong data-chat-settings-summary>Model loading - Medium</strong>
        </button>
        <div class="chat-tool-toggles" aria-label="Tool controls">
          <label class="check"><input type="checkbox" data-chat-search checked> Search</label>
          <label class="check"><input type="checkbox" data-chat-read checked> Read web</label>
          <label class="check"><input type="checkbox" data-chat-process checked> Process</label>
          <button type="button" class="secondary icon-label chat-process-reopen hidden" data-chat-process-reopen title="Show process panel" aria-label="Show process panel"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-left"></use></svg><span>Process</span></button>
        </div>
        <div class="chat-manage-actions">
          <button type="button" class="secondary action-icon" data-chat-auto-title title="Generate title" aria-label="Generate title"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg></button>
          <button type="button" class="secondary action-icon" data-chat-rename title="Rename" aria-label="Rename"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-edit"></use></svg></button>
          <button type="button" class="secondary action-icon" data-chat-delete title="Delete" aria-label="Delete"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-trash"></use></svg></button>
        </div>
      </div>
      <div class="chat-workspace">
        <div class="chat-body" data-chat-messages></div>
        <aside class="chat-process-shell" data-chat-process-shell aria-label="Process timeline">
          <div class="chat-process-head">
            <div class="chat-process-title">
              <strong>Process</strong>
              <span>Thinking and tools</span>
            </div>
            <div class="chat-process-actions">
              <button type="button" class="secondary action-icon" data-chat-process-top title="Scroll to top" aria-label="Scroll to top"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-up"></use></svg></button>
              <button type="button" class="secondary action-icon" data-chat-process-bottom title="Scroll to bottom" aria-label="Scroll to bottom"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-down"></use></svg></button>
              <button type="button" class="secondary action-icon" data-chat-process-collapse aria-expanded="true" title="Collapse process" aria-label="Collapse process"><svg class="icon" aria-hidden="true"><use data-chat-process-collapse-icon href="/admin/static/icons.svg#icon-chevron-right"></use></svg></button>
            </div>
          </div>
          <div class="chat-process" data-chat-process-panel></div>
        </aside>
      </div>
      <form class="chat-composer" data-chat-form>
        <textarea data-chat-input rows="1" placeholder="Ask a question..." required></textarea>
        <div class="chat-actions">
          <button type="button" class="secondary hidden" data-chat-stop>Stop</button>
          <button type="submit">Send</button>
        </div>
      </form>
      <div class="chat-settings-modal hidden" data-chat-settings-modal aria-hidden="true">
        <div class="chat-settings-backdrop" data-chat-settings-close></div>
        <form class="chat-settings-dialog" data-chat-settings-form role="dialog" aria-modal="true" aria-labelledby="admin-chat-settings-title">
          <div class="chat-settings-head">
            <div>
              <h2 id="admin-chat-settings-title">Chat settings</h2>
              <p>Saved to this conversation</p>
            </div>
            <button type="button" class="secondary action-icon" data-chat-settings-close title="Close" aria-label="Close settings"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg></button>
          </div>
          <div class="chat-settings-body">
            <div class="chat-avatar-settings" aria-label="Avatar settings">
              <div class="chat-avatar-card">
                <div class="chat-avatar-card-head">
                  <span>User avatar</span>
                  <input data-chat-user-avatar maxlength="16" placeholder="😀" aria-label="User avatar">
                </div>
                <div class="chat-avatar-picker" aria-label="User avatar presets">
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="user" data-chat-avatar-value="😀">😀</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="user" data-chat-avatar-value="😎">😎</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="user" data-chat-avatar-value="🧑‍💻">🧑‍💻</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="user" data-chat-avatar-value="🧑‍🚀">🧑‍🚀</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="user" data-chat-avatar-value="✨">✨</button>
                </div>
              </div>
              <div class="chat-avatar-card">
                <div class="chat-avatar-card-head">
                  <span>Assistant avatar</span>
                  <input data-chat-assistant-avatar maxlength="16" placeholder="🤖" aria-label="Assistant avatar">
                </div>
                <div class="chat-avatar-picker" aria-label="Assistant avatar presets">
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="assistant" data-chat-avatar-value="🤖">🤖</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="assistant" data-chat-avatar-value="🧠">🧠</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="assistant" data-chat-avatar-value="🛠️">🛠️</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="assistant" data-chat-avatar-value="📚">📚</button>
                  <button type="button" class="chat-avatar-preset" data-chat-avatar-target="assistant" data-chat-avatar-value="🧭">🧭</button>
                </div>
              </div>
            </div>
            <label class="chat-model-field">Model<select data-chat-model></select></label>
            <fieldset class="chat-thinking chat-settings-thinking">
              <legend>Thinking</legend>
              <label><input type="radio" name="admin-chat-thinking" value="off">Off</label>
              <label><input type="radio" name="admin-chat-thinking" value="low">Low</label>
              <label><input type="radio" name="admin-chat-thinking" value="medium" checked>Medium</label>
              <label><input type="radio" name="admin-chat-thinking" value="high">High</label>
            </fieldset>
            <label class="chat-settings-field chat-max-tools-field">Max tool calls<input data-chat-max-tool-calls type="number" min="0" max="20" step="1" value="6"></label>
            <section class="chat-default-system-prompt" aria-label="Default system prompt">
              <div class="chat-default-system-head">
                <strong data-chat-default-system-title>Default system prompt</strong>
                <span data-chat-default-system-hint>Always applied by TokenFlow. Your instructions below are appended.</span>
              </div>
              <pre data-chat-default-system-prompt></pre>
            </section>
            <label class="chat-settings-field">System prompt<textarea data-chat-system-prompt maxlength="8000" rows="8" placeholder="Optional instructions for this conversation"></textarea></label>
            <label class="chat-settings-field">My nickname<input data-chat-nickname maxlength="64" placeholder="Optional display name"></label>
          </div>
          <div class="chat-settings-actions">
            <button type="button" class="secondary" data-chat-settings-cancel>Cancel</button>
            <button type="submit" data-chat-settings-save>Save</button>
          </div>
        </form>
      </div>
    </section>
  </main>
  <script type="module" src="/admin/static/chat/app.js"></script>
</body>
</html>
{{end}}
`
