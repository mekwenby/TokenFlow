package admin

import (
	"encoding/json"
	"errors"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"

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
	Title     string
	Username  string
	CSRFToken string
	Error     string
	Lang      string
	LangJSON  template.JS
	I18NJSON  template.JS
	Path      string
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

type providerModelPricesPayload struct {
	ProviderID int64                     `json:"provider_id"`
	Items      []providerModelPriceInput `json:"items"`
}

type providerModelPriceInput struct {
	Model                           string  `json:"model"`
	InputPriceUSDPerMillion         float64 `json:"input_price_usd_per_million"`
	OutputPriceUSDPerMillion        float64 `json:"output_price_usd_per_million"`
	CacheReadPriceUSDPerMillion     float64 `json:"cache_read_price_usd_per_million"`
	CacheCreationPriceUSDPerMillion float64 `json:"cache_creation_price_usd_per_million"`
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
		sessions: auth.NewSessions(st),
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
	r.Post("/admin/lang", h.languagePost)
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Post("/admin/logout", h.logout)
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
		r.Get("/admin/api/provider-model-prices", h.providerModelPrices)
		r.Put("/admin/api/provider-model-prices", h.providerModelPrices)
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
			BasePath:    "/admin/api/chat",
			Service:     h.chatService,
			Store:       h.store,
			Owner:       h.chatOwner,
			RequireCSRF: h.requireCSRFForWrite,
		})
	})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
	data := h.page(r, "title.dashboard")
	data.Username = session.Username
	_ = h.tpl.ExecuteTemplate(w, "dashboard", data)
}

func (h *Handler) chatPage(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
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
	if err := h.sessions.Create(w, r, user.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	if err := h.sessions.Create(w, r, user.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	if err := h.sessions.RevokeAll(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

func (h *Handler) providerModelPrices(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		providerID, _ := strconv.ParseInt(r.URL.Query().Get("provider_id"), 10, 64)
		if providerID <= 0 {
			providerID = idParam(r)
		}
		if providerID <= 0 {
			h.writeLocalizedError(w, r, http.StatusBadRequest, "detail_id_required")
			return
		}
		prices, err := h.store.ProviderModelPrices(r.Context(), providerID)
		h.writeResult(w, r, prices, err)
	case http.MethodPut:
		var payload providerModelPricesPayload
		if !h.decodePayload(w, r, &payload) {
			return
		}
		if payload.ProviderID <= 0 {
			h.writeLocalizedError(w, r, http.StatusBadRequest, "detail_id_required")
			return
		}
		items := make([]store.ProviderModelPrice, 0, len(payload.Items))
		for _, input := range payload.Items {
			item, ok := providerModelPriceFromInput(w, r, input)
			if !ok {
				return
			}
			items = append(items, item)
		}
		prices, err := h.store.UpdateProviderModelPrices(r.Context(), payload.ProviderID, items)
		if errors.Is(err, store.ErrInvalidPrice) {
			h.writeLocalizedError(w, r, http.StatusBadRequest, "invalid_price")
			return
		}
		h.writeResult(w, r, prices, err)
	}
}

func providerModelPriceFromInput(w http.ResponseWriter, r *http.Request, input providerModelPriceInput) (store.ProviderModelPrice, bool) {
	if input.InputPriceUSDPerMillion < 0 || input.OutputPriceUSDPerMillion < 0 || input.CacheReadPriceUSDPerMillion < 0 || input.CacheCreationPriceUSDPerMillion < 0 {
		writeError(w, http.StatusBadRequest, tr(languageFromRequest(r), "invalid_price"))
		return store.ProviderModelPrice{}, false
	}
	return store.ProviderModelPrice{
		Model:                                strings.TrimSpace(input.Model),
		InputPriceMicroUSDPerMillion:         usdPerMillionToMicroUSD(input.InputPriceUSDPerMillion),
		OutputPriceMicroUSDPerMillion:        usdPerMillionToMicroUSD(input.OutputPriceUSDPerMillion),
		CacheReadPriceMicroUSDPerMillion:     usdPerMillionToMicroUSD(input.CacheReadPriceUSDPerMillion),
		CacheCreationPriceMicroUSDPerMillion: usdPerMillionToMicroUSD(input.CacheCreationPriceUSDPerMillion),
	}, true
}

func usdPerMillionToMicroUSD(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Round(value * 1_000_000))
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
	session, ok := auth.SessionFromRequest(r)
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
		session, err := h.sessions.Authenticate(w, r)
		if errors.Is(err, auth.ErrSessionNotFound) {
			if strings.HasPrefix(r.URL.Path, "/admin/api/") {
				h.writeLocalizedError(w, r, http.StatusUnauthorized, "login_required")
			} else {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			}
			return
		}
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, auth.WithSession(r, session))
	})
}

func (h *Handler) requireCSRFForWrite(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	if h.sessions.ValidateCSRF(r) {
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
	csrf := ""
	if session, ok := auth.SessionFromRequest(r); ok {
		csrf = session.CSRFToken
	} else if cookie, err := r.Cookie(auth.CSRFCookie); err == nil {
		csrf = cookie.Value
	}
	return pageData{
		Title:     tr(lang, titleKey),
		CSRFToken: csrf,
		Lang:      lang,
		LangJSON:  jsonString(lang),
		I18NJSON:  i18nJSON(lang),
		Path:      safeAdminNext(r.URL.RequestURI()),
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
{{define "admin_pwa_head"}}
  <meta name="theme-color" content="#101820">
  <link rel="manifest" href="/admin/manifest.webmanifest">
  <link rel="apple-touch-icon" href="/admin/static/pwa/icon-192.png">
  <script src="/admin/static/pwa/register.js" defer></script>
{{end}}

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
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" type="image/png" href="/admin/static/tokenflow-logo.png"><link rel="stylesheet" href="/admin/static/css/tokens.css">
  {{template "admin_pwa_head" .}}
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/charts.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script></head>
<body class="auth-page">
  <main class="auth-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.png" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
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
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" type="image/png" href="/admin/static/tokenflow-logo.png"><link rel="stylesheet" href="/admin/static/css/tokens.css">
  {{template "admin_pwa_head" .}}
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/charts.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script></head>
<body class="auth-page">
  <main class="auth-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.png" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
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
  <link rel="icon" type="image/png" href="/admin/static/tokenflow-logo.png">
  {{template "admin_pwa_head" .}}
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
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.png" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      <div class="top-actions">
        {{template "lang_switch" .}}
        <span class="user-chip" title="{{.Username}}">{{.Username}}</span>
        <form method="post" action="/admin/logout">
          <input type="hidden" name="csrf" value="{{.CSRFToken}}">
          <button type="submit" class="secondary icon-label">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg>
            <span>{{tr .Lang "logout"}}</span>
          </button>
        </form>
      </div>
    </div>
  </header>
  <div class="app-shell">
    <aside class="side-nav" aria-label="{{tr .Lang "admin_navigation"}}">
      <a class="nav-item" href="/admin/chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chat"></use></svg><span>LLM Chat</span></a>
      <a class="nav-item active" href="#overview" data-nav-view="overview"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "overview"}}</span></a>
      <a class="nav-item" href="#api-addresses" data-nav-view="api-addresses"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>{{tr .Lang "api_addresses"}}</span></a>
      <a class="nav-item" href="#users" data-nav-view="users"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-users"></use></svg><span>{{tr .Lang "users"}}</span></a>
      <a class="nav-item" href="#providers" data-nav-view="providers"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-server"></use></svg><span>{{tr .Lang "providers"}}</span></a>
      <a class="nav-item" href="#mappings" data-nav-view="mappings"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg><span>{{tr .Lang "model_mappings"}}</span></a>
      <a class="nav-item" href="#keys" data-nav-view="keys"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg><span>{{tr .Lang "distribution_keys"}}</span></a>
      <a class="nav-item" href="#logs" data-nav-view="logs"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>{{tr .Lang "recent_requests"}}</span></a>
    </aside>
  <main class="layout">
    <section id="overview" class="panel stats-panel" data-admin-view="overview">
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
    <section id="api-addresses-section" class="panel" data-admin-view="api-addresses" hidden>
      <h2>{{tr .Lang "api_addresses"}}</h2>
      <div id="api-addresses" class="api-grid"></div>
    </section>
    <section id="users-section" class="panel" data-admin-view="users" hidden>
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
    <section id="providers-section" class="panel" data-admin-view="providers" hidden>
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
      <form id="provider-prices-form" class="editor hidden">
        <input type="hidden" name="provider_id">
        <h3>{{tr .Lang "model_prices"}}</h3>
        <div id="provider-prices-editor" class="table-wrap"></div>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-action="cancel">{{tr .Lang "cancel"}}</button></div>
      </form>
      <div id="providers" class="table-wrap"></div>
    </section>
    <section id="mappings-section" class="panel" data-admin-view="mappings" hidden>
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
    <section id="keys-section" class="panel" data-admin-view="keys" hidden>
      <div class="section-head">
        <h2>{{tr .Lang "distribution_keys"}}</h2>
        <div class="section-actions">
          <button type="button" class="icon-label" data-action="open" data-id="key-form">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
            <span>{{tr .Lang "create_key"}}</span>
          </button>
          <button type="button" id="keys-fullscreen-toggle" class="secondary action-icon" data-action="toggle-keys-fullscreen" title="{{tr .Lang "fullscreen"}}" aria-label="{{tr .Lang "fullscreen"}}" aria-pressed="false">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-fullscreen"></use></svg>
          </button>
        </div>
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
    <section id="logs-section" class="panel" data-admin-view="logs" hidden>
      <div class="section-head logs-head">
        <h2>{{tr .Lang "recent_requests"}}</h2>
        <div class="logs-tools">
          <form id="logs-search-form" class="logs-search">
            <input name="q" type="search" placeholder="{{tr .Lang "logs_search_placeholder"}}" aria-label="{{tr .Lang "logs_search"}}">
            <button type="submit" class="action-icon" title="{{tr .Lang "logs_search"}}" aria-label="{{tr .Lang "logs_search"}}">
              <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg>
            </button>
            <button type="button" class="secondary action-icon" data-action="clear-logs-search" title="{{tr .Lang "clear"}}" aria-label="{{tr .Lang "clear"}}">
              <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg>
            </button>
          </form>
          <button type="button" id="logs-fullscreen-toggle" class="secondary action-icon" data-action="toggle-logs-fullscreen" title="{{tr .Lang "fullscreen"}}" aria-label="{{tr .Lang "fullscreen"}}" aria-pressed="false">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-fullscreen"></use></svg>
          </button>
        </div>
      </div>
      <div id="logs" class="table-wrap"></div>
      <div id="logs-pager"></div>
    </section>
    <section id="more-section" class="panel more-view" data-admin-view="more" hidden>
      <h2>{{tr .Lang "more"}}</h2>
      <nav class="more-nav-list" aria-label="{{tr .Lang "more_navigation"}}">
        <a class="more-nav-item" href="#api-addresses">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg>
          <span>{{tr .Lang "api_addresses"}}</span>
          <svg class="icon more-nav-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg>
        </a>
        <a class="more-nav-item" href="#mappings">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg>
          <span>{{tr .Lang "model_mappings"}}</span>
          <svg class="icon more-nav-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg>
        </a>
        <a class="more-nav-item" href="#keys">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg>
          <span>{{tr .Lang "distribution_keys"}}</span>
          <svg class="icon more-nav-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg>
        </a>
        <a class="more-nav-item" href="#logs">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg>
          <span>{{tr .Lang "recent_requests"}}</span>
          <svg class="icon more-nav-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg>
        </a>
      </nav>
    </section>
  </main>
  </div>
  <nav class="mobile-nav" aria-label="{{tr .Lang "admin_navigation"}}">
    <a class="nav-item" href="/admin/chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chat"></use></svg><span>LLM Chat</span></a>
    <a class="nav-item active" href="#overview" data-nav-view="overview"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "overview"}}</span></a>
    <a class="nav-item" href="#users" data-nav-view="users"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-users"></use></svg><span>{{tr .Lang "users"}}</span></a>
    <a class="nav-item" href="#providers" data-nav-view="providers"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-server"></use></svg><span>{{tr .Lang "providers"}}</span></a>
    <a class="nav-item" href="#more" data-nav-view="more" data-nav-active-for="api-addresses mappings keys logs more"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-more"></use></svg><span>{{tr .Lang "more"}}</span></a>
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
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>{{.Title}}</title>
  <link rel="icon" type="image/png" href="/admin/static/tokenflow-logo.png">
  {{template "admin_pwa_head" .}}
  <link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <link rel="stylesheet" href="/admin/static/css/chat.css">
  <script src="/admin/static/theme.js"></script>
</head>
<body class="admin-page chat-page">
  <main class="chat-app" data-chat-root data-chat-ready="false" data-chat-lang="{{.Lang}}" data-chat-api-prefix="/admin/api/chat" data-chat-csrf-cookie="gateway_csrf">
    <button type="button" class="chat-sidebar-backdrop" data-chat-sidebar-backdrop aria-label="Close conversations"></button>
    <aside class="chat-app-sidebar" data-chat-sidebar aria-label="Conversations">
      <div class="chat-sidebar-head">
        <a class="chat-sidebar-brand" href="/admin/chat"><img src="/admin/static/tokenflow-logo.png" alt=""><span>{{tr .Lang "app.title"}}</span></a>
        <div class="chat-sidebar-head-actions">
          <button type="button" class="chat-icon-button" data-chat-new title="New chat" aria-label="New chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg></button>
          <button type="button" class="chat-icon-button" data-chat-sidebar-collapse title="Collapse sidebar" aria-label="Collapse sidebar"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-panel-left-close"></use></svg></button>
        </div>
      </div>
      <label class="chat-conversation-search">
        <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg>
        <input type="search" data-chat-conversation-search placeholder="Search conversations" autocomplete="off">
      </label>
      <div class="chat-list" data-chat-conversations></div>
      <div class="chat-sidebar-foot">
        <button type="button" class="chat-account-button" data-chat-account-toggle aria-expanded="false">
          <span class="chat-account-avatar" aria-hidden="true">{{.Username}}</span>
          <span class="chat-account-name">{{.Username}}</span>
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-more"></use></svg>
        </button>
        <div class="chat-popover chat-account-menu hidden" data-chat-account-menu>
          <a href="/admin"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "overview"}}</span></a>
          {{template "lang_switch" .}}
          <form method="post" action="/admin/logout"><input type="hidden" name="csrf" value="{{.CSRFToken}}"><button type="submit"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg><span>{{tr .Lang "logout"}}</span></button></form>
        </div>
      </div>
    </aside>
    <section class="chat-app-main" aria-label="LLM Chat">
      <header class="chat-main-header">
        <button type="button" class="chat-icon-button" data-chat-sidebar-toggle aria-expanded="true" title="Show conversations" aria-label="Show conversations"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-panel-left"></use></svg></button>
        <div class="chat-header-title">
          <h1 data-chat-title>New chat</h1>
          <button type="button" class="chat-settings-summary" data-chat-settings-open title="Chat settings" aria-label="Chat settings"><span data-chat-settings-summary>Model loading - Medium</span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-down"></use></svg></button>
        </div>
        <div class="chat-header-menu-wrap">
          <button type="button" class="chat-icon-button" data-chat-conversation-menu-toggle aria-expanded="false" title="Conversation actions" aria-label="Conversation actions"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-more"></use></svg></button>
          <div class="chat-popover chat-conversation-menu hidden" data-chat-conversation-menu>
            <button type="button" data-chat-auto-title><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg><span>Generate title</span></button>
            <button type="button" data-chat-rename><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-edit"></use></svg><span>Rename</span></button>
            <button type="button" class="danger-text" data-chat-delete><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-trash"></use></svg><span>Delete</span></button>
          </div>
        </div>
      </header>
      <div class="chat-body" data-chat-messages></div>
      <button type="button" class="chat-scroll-bottom hidden" data-chat-scroll-bottom title="Scroll to bottom" aria-label="Scroll to bottom"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-down"></use></svg></button>
      <div class="chat-composer-shell">
        <form class="chat-composer" data-chat-form>
          <textarea data-chat-input rows="1" placeholder="Ask a question..." required></textarea>
          <div class="chat-composer-bar">
            <div class="chat-tools-wrap">
              <button type="button" class="chat-icon-button chat-tools-toggle" data-chat-tools-toggle aria-expanded="false" title="Tools" aria-label="Tools"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg></button>
              <div class="chat-popover chat-tools-menu hidden" data-chat-tools-menu>
                <label><input type="checkbox" data-chat-search checked><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg><span>Search</span></label>
                <label><input type="checkbox" data-chat-read checked><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>Read web</span></label>
                <label><input type="checkbox" data-chat-process checked><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>Process</span></label>
              </div>
              <div class="chat-tool-status" data-chat-tool-status></div>
            </div>
            <div class="chat-send-actions">
              <button type="button" class="chat-send-button hidden" data-chat-stop title="Stop" aria-label="Stop"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-stop"></use></svg></button>
              <button type="submit" class="chat-send-button" data-chat-send title="Send" aria-label="Send"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-up"></use></svg></button>
            </div>
          </div>
        </form>
      </div>
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
            <section class="chat-settings-section">
              <h3 data-chat-model-section-title>Model and reasoning</h3>
              <label class="chat-model-field">Model<select data-chat-model></select></label>
              <fieldset class="chat-thinking chat-settings-thinking"><legend>Thinking</legend><label><input type="radio" name="admin-chat-thinking" value="off">Off</label><label><input type="radio" name="admin-chat-thinking" value="low">Low</label><label><input type="radio" name="admin-chat-thinking" value="medium" checked>Medium</label><label><input type="radio" name="admin-chat-thinking" value="high">High</label></fieldset>
              <label class="chat-settings-field chat-max-tools-field">Max tool calls<input data-chat-max-tool-calls type="number" min="0" max="20" step="1" value="7"></label>
            </section>
            <section class="chat-settings-section">
              <h3 data-chat-instructions-title>Instructions</h3>
              <details class="chat-default-system-prompt"><summary data-chat-default-system-title>Default system prompt</summary><span data-chat-default-system-hint>Always applied by TokenFlow. Your instructions below are appended.</span><pre data-chat-default-system-prompt></pre></details>
              <label class="chat-settings-field">System prompt<textarea data-chat-system-prompt maxlength="8000" rows="7" placeholder="Optional instructions for this conversation"></textarea></label>
            </section>
            <section class="chat-settings-section">
              <h3 data-chat-identity-title>Identity</h3>
              <label class="chat-settings-field">My nickname<input data-chat-nickname maxlength="64" placeholder="Optional display name"></label>
              <div class="chat-avatar-settings" aria-label="Avatar settings">
                <div class="chat-avatar-field"><div class="chat-avatar-card-head"><span>User avatar</span><input data-chat-user-avatar maxlength="16" placeholder="😀" aria-label="User avatar"></div><div class="chat-avatar-picker" aria-label="User avatar presets"></div></div>
                <div class="chat-avatar-field"><div class="chat-avatar-card-head"><span>Assistant avatar</span><input data-chat-assistant-avatar maxlength="16" placeholder="🤖" aria-label="Assistant avatar"></div><div class="chat-avatar-picker" aria-label="Assistant avatar presets"></div></div>
              </div>
            </section>
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
