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
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
	"tokenflow/web"
)

type Handler struct {
	store    *store.Store
	box      *secret.Box
	sessions *auth.Sessions
	tpl      *template.Template
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
		r.Get("/admin/api/stats", h.stats)
		r.Get("/admin/api/token-usage", h.tokenUsage)
		r.Get("/admin/api/model-token-details", h.modelTokenDetails)
		r.Get("/admin/api/logs", h.logs)
	})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessions.Get(r)
	data := h.page(r, "title.dashboard")
	data.Username = session.Username
	_ = h.tpl.ExecuteTemplate(w, "dashboard", data)
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
			status = http.StatusNotFound
			h.writeLocalizedError(w, r, status, "not_found")
			return
		}
		writeError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) writeLocalizedError(w http.ResponseWriter, r *http.Request, status int, key string) {
	writeError(w, status, tr(languageFromRequest(r), key))
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	return id
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
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" href="/admin/static/tokenflow-logo.svg"><link rel="stylesheet" href="/admin/static/style.css"></head>
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
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" href="/admin/static/tokenflow-logo.svg"><link rel="stylesheet" href="/admin/static/style.css"></head>
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
  <link rel="stylesheet" href="/admin/static/style.css">
</head>
<body class="admin-page">
  <header class="topbar">
    <div class="topbar-inner">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      <div class="top-actions">
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
  <main class="layout">
    <section class="panel stats-panel">
      <h2>{{tr .Lang "overview"}}</h2>
      <div id="stats" class="stats-grid"></div>
      <div class="usage-section">
        <div class="section-head usage-head">
          <h3>{{tr .Lang "token_usage"}}</h3>
          <div class="segmented" role="group" aria-label="{{tr .Lang "usage_range"}}">
            <button type="button" data-usage-range="24h">{{tr .Lang "last_24_hours"}}</button>
            <button type="button" data-usage-range="7d">{{tr .Lang "last_7_days"}}</button>
          </div>
        </div>
        <div id="token-usage" class="usage-chart"></div>
      </div>
    </section>
    <section class="panel">
      <h2>{{tr .Lang "api_addresses"}}</h2>
      <div id="api-addresses" class="api-grid"></div>
    </section>
    <section class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "providers"}}</h2>
        <button type="button" class="icon-label" data-open="provider-form">
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
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-cancel>{{tr .Lang "cancel"}}</button></div>
      </form>
      <div id="providers" class="table-wrap"></div>
    </section>
    <section class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "model_mappings"}}</h2>
        <button type="button" class="icon-label" data-open="mapping-form">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
          <span>{{tr .Lang "add_mapping"}}</span>
        </button>
      </div>
      <form id="mapping-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "client_model"}}<input name="client_model" required></label>
        <label>{{tr .Lang "provider"}}<select name="provider_id" required></select></label>
        <label>{{tr .Lang "upstream_model"}}<input name="upstream_model" required></label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-cancel>{{tr .Lang "cancel"}}</button></div>
      </form>
      <div id="mappings" class="table-wrap"></div>
    </section>
    <section class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "distribution_keys"}}</h2>
        <button type="button" class="icon-label" data-open="key-form">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
          <span>{{tr .Lang "create_key"}}</span>
        </button>
      </div>
      <form id="key-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "name"}}<input name="name" required></label>
        <label class="check key-enabled hidden"><input name="enabled" type="checkbox" checked> {{tr .Lang "enabled"}}</label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-cancel>{{tr .Lang "cancel"}}</button></div>
      </form>
      <p id="new-key" class="secret hidden"></p>
      <div id="keys" class="table-wrap"></div>
    </section>
    <section class="panel">
      <div class="section-head logs-head">
        <h2>{{tr .Lang "recent_requests"}}</h2>
        <form id="logs-search-form" class="logs-search">
          <input name="q" type="search" placeholder="{{tr .Lang "logs_search_placeholder"}}" aria-label="{{tr .Lang "logs_search"}}">
          <button type="submit" class="action-icon" title="{{tr .Lang "logs_search"}}" aria-label="{{tr .Lang "logs_search"}}">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg>
          </button>
          <button type="button" class="secondary action-icon" data-clear-logs-search title="{{tr .Lang "clear"}}" aria-label="{{tr .Lang "clear"}}">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg>
          </button>
        </form>
      </div>
      <div id="logs" class="table-wrap"></div>
    </section>
  </main>
  <div id="detail-modal" class="modal hidden" role="dialog" aria-modal="true" aria-labelledby="detail-modal-title">
    <div class="modal-dialog">
      <div class="modal-head">
        <div>
          <h2 id="detail-modal-title">{{tr .Lang "model_token_details"}}</h2>
          <p id="detail-modal-subtitle"></p>
        </div>
        <button type="button" class="secondary icon-label" data-close-detail>
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
  <script src="/admin/static/app.js"></script>
</body>
</html>
{{end}}
`
