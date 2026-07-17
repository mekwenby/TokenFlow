package account

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"tokenflow/internal/auth"
	"tokenflow/internal/chat"
	"tokenflow/internal/httputil"
	"tokenflow/internal/store"
)

const (
	sessionCookie   = "gateway_account_session"
	csrfCookie      = "gateway_account_csrf"
	adminLangCookie = "gateway_lang"
)

type Handler struct {
	store       *store.Store
	chatService *chat.Service
	sessions    *auth.Sessions
	tpl         *template.Template
}

type pageData struct {
	Title     string
	Email     string
	CSRFToken string
	Error     string
	Message   string
	User      store.ConsumerUser
	Lang      string
	LangJSON  template.JS
	I18NJSON  template.JS
}

type keyPayload struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type accountLogItem struct {
	CreatedAt           time.Time `json:"created_at"`
	Protocol            string    `json:"protocol"`
	Model               string    `json:"model"`
	DistributionKeyID   *int64    `json:"distribution_key_id,omitempty"`
	DistributionKeyName string    `json:"distribution_key_name,omitempty"`
	StatusCode          int       `json:"status_code"`
	LatencyMS           int64     `json:"latency_ms"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheHitRate        float64   `json:"cache_hit_rate"`
	Stream              bool      `json:"stream"`
}

type accountLogsPage struct {
	Items  []accountLogItem `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
	Query  string           `json:"query"`
}

const (
	defaultLogsLimit = 50
	maxLogsLimit     = 200
)

func New(st *store.Store) *Handler {
	return &Handler{
		store:    st,
		sessions: auth.NewScopedSessions(st, store.AuthSessionOwnerConsumer, sessionCookie, csrfCookie, "/account"),
		tpl: template.Must(template.New("account").Funcs(template.FuncMap{
			"tr":              accountTr,
			"accountI18NJSON": accountI18NJSON,
			"compactNumber":   compactNumber,
			"jsonString":      jsonString,
		}).Parse(templates)),
	}
}

func (h *Handler) SetChatService(service *chat.Service) {
	h.chatService = service
}

func compactNumber(value int64) string {
	if value < 1000 {
		if value < 0 {
			return "0"
		}
		return strconv.FormatInt(value, 10)
	}
	units := []struct {
		divisor int64
		suffix  string
	}{
		{1000000000, "B"},
		{1000000, "M"},
		{1000, "K"},
	}
	for _, unit := range units {
		if value >= unit.divisor {
			out := strconv.FormatFloat(float64(value)/float64(unit.divisor), 'f', 1, 64)
			return strings.TrimSuffix(out, ".0") + unit.suffix
		}
	}
	return strconv.FormatInt(value, 10)
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/account/register", h.registerForm)
	r.Post("/account/register", h.registerPost)
	r.Get("/account/login", h.loginForm)
	r.Post("/account/login", h.loginPost)
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Post("/account/logout", h.logout)
		r.Get("/account", h.dashboard)
		r.Get("/account/chat", h.chatPage)
		r.Get("/account/api/keys", h.keys)
		r.Post("/account/api/keys", h.keys)
		r.Put("/account/api/keys", h.keys)
		r.Delete("/account/api/keys", h.keys)
		r.Post("/account/api/keys/reset", h.resetKey)
		r.Get("/account/api/logs", h.logs)
		chat.RegisterRoutes(r, chat.RouteConfig{
			BasePath:    "/account/api/chat",
			Service:     h.chatService,
			Store:       h.store,
			Owner:       h.chatOwner,
			RequireCSRF: h.requireCSRFForWrite,
		})
	})
}

func (h *Handler) registerForm(w http.ResponseWriter, r *http.Request) {
	_ = h.tpl.ExecuteTemplate(w, "register", h.page(r, "title.register"))
}

func (h *Handler) registerPost(w http.ResponseWriter, r *http.Request) {
	email := normalizeEmail(r.FormValue("email"))
	password := r.FormValue("password")
	if !validEmail(email) || len(password) < 8 {
		data := h.page(r, "title.register")
		data.Email = email
		data.Error = accountTr(data.Lang, "register_invalid")
		_ = h.tpl.ExecuteTemplate(w, "register", data)
		return
	}
	if _, err := h.store.ConsumerUserByEmail(r.Context(), email); err == nil {
		data := h.page(r, "title.register")
		data.Email = email
		data.Error = accountTr(data.Lang, "register_duplicate")
		_ = h.tpl.ExecuteTemplate(w, "register", data)
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := h.store.CreateConsumerUser(r.Context(), email, string(hash)); err != nil {
		data := h.page(r, "title.register")
		data.Email = email
		data.Error = err.Error()
		_ = h.tpl.ExecuteTemplate(w, "register", data)
		return
	}
	data := h.page(r, "title.login")
	data.Message = accountTr(data.Lang, "registration_submitted")
	_ = h.tpl.ExecuteTemplate(w, "login", data)
}

func (h *Handler) loginForm(w http.ResponseWriter, r *http.Request) {
	_ = h.tpl.ExecuteTemplate(w, "login", h.page(r, "title.login"))
}

func (h *Handler) loginPost(w http.ResponseWriter, r *http.Request) {
	email := normalizeEmail(r.FormValue("email"))
	password := r.FormValue("password")
	user, err := h.store.ConsumerUserByEmail(r.Context(), email)
	if err != nil || user.Status != store.ConsumerStatusEnabled || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		data := h.page(r, "title.login")
		data.Email = email
		data.Error = accountTr(data.Lang, "login_invalid")
		_ = h.tpl.ExecuteTemplate(w, "login", data)
		return
	}
	if err := h.sessions.Create(w, r, user.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	if err := h.sessions.RevokeAll(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/account/login", http.StatusSeeOther)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
	user, err := h.store.ConsumerUser(r.Context(), session.UserID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err != nil || user.Status != store.ConsumerStatusEnabled {
		h.sessions.ClearCookies(w, r)
		http.Redirect(w, r, "/account/login", http.StatusSeeOther)
		return
	}
	data := h.page(r, "title.dashboard")
	data.Email = session.Username
	data.User = user
	_ = h.tpl.ExecuteTemplate(w, "dashboard", data)
}

func (h *Handler) chatPage(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
	user, err := h.store.ConsumerUser(r.Context(), session.UserID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err != nil || user.Status != store.ConsumerStatusEnabled {
		h.sessions.ClearCookies(w, r)
		http.Redirect(w, r, "/account/login", http.StatusSeeOther)
		return
	}
	data := h.page(r, "title.dashboard")
	data.Title = "LLM Chat"
	data.Email = session.Username
	data.User = user
	_ = h.tpl.ExecuteTemplate(w, "chat", data)
}

func (h *Handler) keys(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		keys, err := h.store.ConsumerDistributionKeys(r.Context(), session.UserID)
		h.writeResult(w, r, keys, err)
	case http.MethodPost:
		var payload keyPayload
		if !decodePayload(w, r, &payload) {
			return
		}
		plain, prefix, hash, err := auth.NewDistributionKey()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			name = accountTr(accountLanguageFromRequest(r), "api_key_default_name")
		}
		key, err := h.store.CreateConsumerDistributionKey(r.Context(), session.UserID, name, prefix, hash)
		key.PlainKey = plain
		h.writeResult(w, r, key, err)
	case http.MethodPut:
		var payload keyPayload
		if !decodePayload(w, r, &payload) {
			return
		}
		key, err := h.store.UpdateConsumerDistributionKey(r.Context(), session.UserID, payload.ID, strings.TrimSpace(payload.Name), payload.Enabled)
		h.writeResult(w, r, key, err)
	case http.MethodDelete:
		id := idParam(r)
		h.writeResult(w, r, map[string]bool{"ok": true}, h.store.DeleteConsumerDistributionKey(r.Context(), session.UserID, id))
	}
}

func (h *Handler) resetKey(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	var payload keyPayload
	if !decodePayload(w, r, &payload) {
		return
	}
	plain, prefix, hash, err := auth.NewDistributionKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	key, err := h.store.ResetConsumerDistributionKey(r.Context(), session.UserID, payload.ID, prefix, hash)
	key.PlainKey = plain
	h.writeResult(w, r, key, err)
}

func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.SessionFromRequest(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, offset = normalizeLogsParams(limit, offset)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	logs, err := h.store.ConsumerLogsSearch(r.Context(), session.UserID, limit, offset, query)
	if err != nil {
		h.writeResult(w, r, nil, err)
		return
	}
	total, err := h.store.ConsumerLogCountSearch(r.Context(), session.UserID, query)
	if err != nil {
		h.writeResult(w, r, nil, err)
		return
	}
	items := make([]accountLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, accountLogFromStore(log))
	}
	h.writeResult(w, r, accountLogsPage{Items: items, Total: total, Limit: limit, Offset: offset, Query: query}, nil)
}

func (h *Handler) chatOwner(r *http.Request) (store.ChatOwner, bool) {
	session, ok := auth.SessionFromRequest(r)
	if !ok {
		return store.ChatOwner{}, false
	}
	return store.ChatOwner{Type: store.ChatOwnerConsumer, ID: session.UserID, Name: session.Username}, true
}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.sessions.Authenticate(w, r)
		if errors.Is(err, auth.ErrSessionNotFound) {
			if strings.HasPrefix(r.URL.Path, "/account/api/") {
				h.writeLocalizedError(w, r, http.StatusUnauthorized, "login_required")
			} else {
				http.Redirect(w, r, "/account/login", http.StatusSeeOther)
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

func (h *Handler) writeLocalizedError(w http.ResponseWriter, r *http.Request, status int, key string) {
	httputil.WriteError(w, status, accountTr(accountLanguageFromRequest(r), key))
}

func (h *Handler) writeResult(w http.ResponseWriter, r *http.Request, body any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			h.writeLocalizedError(w, r, http.StatusNotFound, "not_found")
			return
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func decodePayload(w http.ResponseWriter, r *http.Request, dst any) bool {
	return httputil.DecodePayload(w, r, dst)
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

func accountLogFromStore(log store.RequestLog) accountLogItem {
	return accountLogItem{
		CreatedAt:           log.CreatedAt,
		Protocol:            log.Protocol,
		Model:               log.Model,
		DistributionKeyID:   log.DistributionKeyID,
		DistributionKeyName: log.DistributionKeyName,
		StatusCode:          log.StatusCode,
		LatencyMS:           log.LatencyMS,
		InputTokens:         log.InputTokens,
		OutputTokens:        log.OutputTokens,
		CacheReadTokens:     log.CacheReadTokens,
		CacheCreationTokens: log.CacheCreationTokens,
		CacheHitRate:        log.CacheHitRate,
		Stream:              log.Stream,
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

var accountTranslations = map[string]map[string]string{
	"en": {
		"app.title":               "TokenFlow",
		"title.register":          "Create account",
		"title.login":             "Account login",
		"title.dashboard":         "TokenFlow Account",
		"create_account":          "Create account",
		"account_login":           "Account login",
		"email":                   "Email",
		"password":                "Password",
		"register":                "Register",
		"login":                   "Log in",
		"logout":                  "Log out",
		"registration_submitted":  "Registration submitted. Wait for an administrator to enable your account and assign quota.",
		"register_invalid":        "Use a valid email and a password with at least 8 characters.",
		"register_duplicate":      "This email is already registered.",
		"login_invalid":           "Invalid credentials or account is not enabled.",
		"usage":                   "Usage",
		"quota":                   "Quota",
		"used":                    "Used",
		"remaining":               "Remaining",
		"requests":                "Requests",
		"recent_requests":         "Recent requests",
		"logs_search":             "Search requests",
		"logs_search_placeholder": "Key / model",
		"clear":                   "Clear",
		"time":                    "Time",
		"key":                     "Key",
		"client_model":            "Client model",
		"cache_hit_rate":          "Cache hit rate",
		"latency":                 "Latency",
		"previous_page":           "Previous page",
		"next_page":               "Next page",
		"showing":                 "Showing",
		"of":                      "of",
		"api_addresses":           "API addresses",
		"api_base":                "API base",
		"openai_chat":             "OpenAI chat",
		"openai_models":           "OpenAI models",
		"anthropic_messages":      "Anthropic messages",
		"anthropic_models":        "Anthropic models",
		"api_keys":                "API keys",
		"create_key":              "Create key",
		"name":                    "Name",
		"prefix":                  "Prefix",
		"status":                  "Status",
		"enabled":                 "Enabled",
		"disabled":                "Disabled",
		"input_tokens":            "Input tokens",
		"cache_read_tokens":       "Cache read tokens",
		"output_tokens":           "Output tokens",
		"last_used":               "Last used",
		"edit":                    "Edit",
		"delete":                  "Delete",
		"regenerate":              "Regenerate",
		"save":                    "Save",
		"cancel":                  "Cancel",
		"new_key":                 "New key",
		"empty":                   "No records yet.",
		"copy":                    "Copy",
		"copied":                  "Copied",
		"confirm_title":           "Confirm action",
		"delete_key_confirm":      "Delete this key?",
		"reset_key_confirm":       "Regenerate this key? The previous key stops working immediately.",
		"loading":                 "Loading...",
		"saving":                  "Saving...",
		"saved":                   "Saved",
		"request_failed":          "Request failed",
		"login_required":          "login required",
		"csrf_invalid":            "invalid CSRF token",
		"api_key_default_name":    "API key",
	},
	"zh-CN": {
		"recent_requests":         "最近请求",
		"logs_search":             "搜索请求",
		"logs_search_placeholder": "Key / 模型",
		"clear":                   "清空",
		"time":                    "时间",
		"key":                     "Key",
		"client_model":            "客户端模型",
		"cache_hit_rate":          "缓存命中率",
		"latency":                 "延迟",
		"previous_page":           "上一页",
		"next_page":               "下一页",
		"showing":                 "显示",
		"of":                      "共",
		"app.title":               "一念通流 TokenFlow",
		"title.register":          "创建账号",
		"title.login":             "账号登录",
		"title.dashboard":         "一念通流 TokenFlow 账号",
		"create_account":          "创建账号",
		"account_login":           "账号登录",
		"email":                   "邮箱",
		"password":                "密码",
		"register":                "注册",
		"login":                   "登录",
		"logout":                  "退出登录",
		"registration_submitted":  "注册已提交，请等待管理员启用账号并分配额度。",
		"register_invalid":        "请输入有效邮箱，密码至少需要 8 个字符。",
		"register_duplicate":      "这个邮箱已经注册。",
		"login_invalid":           "登录信息错误，或账号尚未启用。",
		"usage":                   "使用情况",
		"quota":                   "额度",
		"used":                    "已用",
		"remaining":               "剩余",
		"requests":                "请求数",
		"api_addresses":           "API 地址",
		"api_base":                "API 基础地址",
		"openai_chat":             "OpenAI 对话",
		"openai_models":           "OpenAI 模型列表",
		"anthropic_messages":      "Anthropic 消息",
		"anthropic_models":        "Anthropic 模型列表",
		"api_keys":                "API Keys",
		"create_key":              "创建 Key",
		"name":                    "名称",
		"prefix":                  "前缀",
		"status":                  "状态",
		"enabled":                 "已启用",
		"disabled":                "已禁用",
		"input_tokens":            "输入 Token",
		"cache_read_tokens":       "缓存命中 Token",
		"output_tokens":           "输出 Token",
		"last_used":               "最后使用",
		"edit":                    "编辑",
		"delete":                  "删除",
		"regenerate":              "重新生成",
		"save":                    "保存",
		"cancel":                  "取消",
		"new_key":                 "新 Key",
		"empty":                   "暂无记录。",
		"copy":                    "复制",
		"copied":                  "已复制",
		"confirm_title":           "确认操作",
		"delete_key_confirm":      "确定删除这个 Key？",
		"reset_key_confirm":       "重新生成后旧 Key 会立即失效，确定继续？",
		"loading":                 "加载中...",
		"saving":                  "保存中...",
		"saved":                   "已保存",
		"request_failed":          "请求失败",
		"login_required":          "需要先登录",
		"csrf_invalid":            "CSRF Token 无效",
		"api_key_default_name":    "API Key",
	},
}

func (h *Handler) page(r *http.Request, titleKey string) pageData {
	lang := accountLanguageFromRequest(r)
	csrf := ""
	if session, ok := auth.SessionFromRequest(r); ok {
		csrf = session.CSRFToken
	} else if cookie, err := r.Cookie(csrfCookie); err == nil {
		csrf = cookie.Value
	}
	return pageData{
		Title:     accountTr(lang, titleKey),
		CSRFToken: csrf,
		Lang:      lang,
		LangJSON:  jsonString(lang),
		I18NJSON:  accountI18NJSON(lang),
	}
}

func accountLanguageFromRequest(r *http.Request) string {
	return httputil.LanguageFromRequest(r, adminLangCookie)
}

func normalizeLang(value string) (string, bool) {
	return httputil.NormalizeLang(value)
}

func accountTr(lang, key string) string {
	if values, ok := accountTranslations[lang]; ok {
		if value, ok := values[key]; ok {
			return value
		}
	}
	if value, ok := accountTranslations["en"][key]; ok {
		return value
	}
	return key
}

func accountI18NJSON(lang string) template.JS {
	values := map[string]string{}
	for key, value := range accountTranslations["en"] {
		values[key] = value
	}
	for key, value := range accountTranslations[lang] {
		values[key] = value
	}
	raw, _ := json.Marshal(values)
	return template.JS(raw)
}

func jsonString(value string) template.JS {
	raw, _ := json.Marshal(value)
	return template.JS(raw)
}

const templates = `
{{define "pwa_head"}}
  <meta name="theme-color" content="#101820">
  <link rel="manifest" href="/manifest.webmanifest">
  <link rel="apple-touch-icon" href="/admin/static/pwa/icon-192.png">
  <script src="/admin/static/pwa/register.js" defer></script>
{{end}}

{{define "auth_head"}}
<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" href="/admin/static/tokenflow-logo.svg"><link rel="stylesheet" href="/admin/static/css/tokens.css">
  {{template "pwa_head" .}}
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/charts.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <script src="/admin/static/theme.js"></script></head>
{{end}}

{{define "register"}}
{{template "auth_head" .}}
<body class="auth-page">
  <main class="auth-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      <a href="/account/login">{{tr .Lang "login"}}</a>
    </div>
    <h2 class="auth-title">{{tr .Lang "create_account"}}</h2>
    <form method="post" action="/account/register">
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <label>{{tr .Lang "email"}}<input name="email" type="email" autocomplete="email" value="{{.Email}}" required></label>
      <label>{{tr .Lang "password"}}<input name="password" type="password" autocomplete="new-password" minlength="8" required></label>
      <button type="submit">{{tr .Lang "register"}}</button>
    </form>
  </main>
</body>
</html>
{{end}}

{{define "login"}}
{{template "auth_head" .}}
<body class="auth-page">
  <main class="auth-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>{{tr .Lang "app.title"}}</span></h1>
      <a href="/account/register">{{tr .Lang "register"}}</a>
    </div>
    <h2 class="auth-title">{{tr .Lang "account_login"}}</h2>
    <form method="post" action="/account/login">
      {{if .Message}}<p class="secret">{{.Message}}</p>{{end}}
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <label>{{tr .Lang "email"}}<input name="email" type="email" autocomplete="email" value="{{.Email}}" required></label>
      <label>{{tr .Lang "password"}}<input name="password" type="password" autocomplete="current-password" required></label>
      <button type="submit">{{tr .Lang "login"}}</button>
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
  {{template "pwa_head" .}}
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
        <span class="user-chip" title="{{.Email}}">{{.Email}}</span>
        <form method="post" action="/account/logout"><input type="hidden" name="csrf" value="{{.CSRFToken}}"><button type="submit" class="secondary icon-label"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg><span>{{tr .Lang "logout"}}</span></button></form>
      </div>
    </div>
  </header>
  <div class="app-shell account-shell">
    <aside class="side-nav" aria-label="Account sections">
      <a class="nav-item" href="/account/chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chat"></use></svg><span>LLM Chat</span></a>
      <a class="nav-item active" href="#account-usage-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "usage"}}</span></a>
      <a class="nav-item" href="#account-api-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>{{tr .Lang "api_addresses"}}</span></a>
      <a class="nav-item" href="#account-keys-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg><span>{{tr .Lang "api_keys"}}</span></a>
      <a class="nav-item" href="#account-logs-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>{{tr .Lang "recent_requests"}}</span></a>
    </aside>
  <main class="layout">
    <section id="account-usage-section" class="panel stats-panel">
      <h2>{{tr .Lang "usage"}}</h2>
      <div class="stats-grid">
        <div class="stat"><span>{{tr .Lang "quota"}}</span><strong>{{compactNumber .User.QuotaTotalTokens}}</strong></div>
        <div class="stat"><span>{{tr .Lang "used"}}</span><strong>{{compactNumber .User.QuotaUsedTokens}}</strong></div>
        <div class="stat"><span>{{tr .Lang "remaining"}}</span><strong>{{compactNumber .User.QuotaRemainingTokens}}</strong></div>
        <div class="stat"><span>{{tr .Lang "requests"}}</span><strong>{{compactNumber .User.RequestCount}}</strong></div>
      </div>
    </section>
    <section id="account-api-section" class="panel">
      <h2>{{tr .Lang "api_addresses"}}</h2>
      <div class="api-grid" id="account-api-addresses"></div>
    </section>
    <section id="account-keys-section" class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "api_keys"}}</h2>
        <button type="button" class="icon-label" data-action="open-account-key"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg><span>{{tr .Lang "create_key"}}</span></button>
      </div>
      <form id="account-key-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "name"}}<input name="name" required></label>
        <label class="check account-key-enabled hidden"><input name="enabled" type="checkbox" checked> {{tr .Lang "enabled"}}</label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-action="cancel-account-key">{{tr .Lang "cancel"}}</button></div>
      </form>
      <p id="account-new-key" class="secret hidden"></p>
      <div id="account-keys" class="table-wrap"></div>
    </section>
    <section id="account-logs-section" class="panel">
      <div class="section-head logs-head">
        <h2>{{tr .Lang "recent_requests"}}</h2>
        <form id="account-logs-search-form" class="logs-search">
          <input name="q" type="search" placeholder="{{tr .Lang "logs_search_placeholder"}}" aria-label="{{tr .Lang "logs_search"}}">
          <button type="submit" class="action-icon" title="{{tr .Lang "logs_search"}}" aria-label="{{tr .Lang "logs_search"}}">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg>
          </button>
          <button type="button" class="secondary" data-action="clear-account-logs-search">{{tr .Lang "clear"}}</button>
        </form>
      </div>
      <div id="account-logs" class="table-wrap"></div>
      <div id="account-logs-pager"></div>
    </section>
  </main>
  </div>
  <nav class="mobile-nav" aria-label="Account sections">
    <a class="nav-item" href="/account/chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chat"></use></svg><span>LLM Chat</span></a>
    <a class="nav-item active" href="#account-usage-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "usage"}}</span></a>
    <a class="nav-item" href="#account-api-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>{{tr .Lang "api_addresses"}}</span></a>
    <a class="nav-item" href="#account-keys-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg><span>{{tr .Lang "api_keys"}}</span></a>
    <a class="nav-item" href="#account-logs-section" data-nav-link><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>{{tr .Lang "recent_requests"}}</span></a>
  </nav>
  <script>
    window.__ACCOUNT_LANG__ = {{.LangJSON}};
    window.__ACCOUNT_I18N__ = {{.I18NJSON}};
  </script>
  <script type="module" src="/admin/static/account/app.js"></script>
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
  <link rel="icon" href="/admin/static/tokenflow-logo.svg">
  {{template "pwa_head" .}}
  <link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <link rel="stylesheet" href="/admin/static/css/chat.css">
  <script src="/admin/static/theme.js"></script>
</head>
<body class="admin-page chat-page">
  <main class="chat-app" data-chat-root data-chat-ready="false" data-chat-lang="{{.Lang}}" data-chat-api-prefix="/account/api/chat" data-chat-csrf-cookie="gateway_account_csrf" data-chat-settings-writable="false">
    <button type="button" class="chat-sidebar-backdrop" data-chat-sidebar-backdrop aria-label="Close conversations"></button>
    <aside class="chat-app-sidebar" data-chat-sidebar aria-label="Conversations">
      <div class="chat-sidebar-head">
        <a class="chat-sidebar-brand" href="/account/chat"><img src="/admin/static/tokenflow-logo.svg" alt=""><span>{{tr .Lang "app.title"}}</span></a>
        <div class="chat-sidebar-head-actions">
          <button type="button" class="chat-icon-button" data-chat-new title="New chat" aria-label="New chat"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg></button>
          <button type="button" class="chat-icon-button" data-chat-sidebar-collapse title="Collapse sidebar" aria-label="Collapse sidebar"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-panel-left-close"></use></svg></button>
        </div>
      </div>
      <label class="chat-conversation-search"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg><input type="search" data-chat-conversation-search placeholder="Search conversations" autocomplete="off"></label>
      <div class="chat-list" data-chat-conversations></div>
      <div class="chat-sidebar-foot">
        <button type="button" class="chat-account-button" data-chat-account-toggle aria-expanded="false"><span class="chat-account-avatar" aria-hidden="true">{{.Email}}</span><span class="chat-account-name">{{.Email}}</span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-more"></use></svg></button>
        <div class="chat-popover chat-account-menu hidden" data-chat-account-menu>
          <a href="/account"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-overview"></use></svg><span>{{tr .Lang "usage"}}</span></a>
          <form method="post" action="/account/logout"><input type="hidden" name="csrf" value="{{.CSRFToken}}"><button type="submit"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg><span>{{tr .Lang "logout"}}</span></button></form>
        </div>
      </div>
    </aside>
    <section class="chat-app-main" aria-label="LLM Chat">
      <header class="chat-main-header">
        <button type="button" class="chat-icon-button" data-chat-sidebar-toggle aria-expanded="true" title="Show conversations" aria-label="Show conversations"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-panel-left"></use></svg></button>
        <div class="chat-header-title"><h1 data-chat-title>New chat</h1><button type="button" class="chat-settings-summary" data-chat-settings-open title="Chat settings" aria-label="Chat settings"><span data-chat-settings-summary>Model loading - Medium</span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-down"></use></svg></button></div>
        <div class="chat-header-menu-wrap">
          <button type="button" class="chat-icon-button" data-chat-conversation-menu-toggle aria-expanded="false" title="Conversation actions" aria-label="Conversation actions"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-more"></use></svg></button>
          <div class="chat-popover chat-conversation-menu hidden" data-chat-conversation-menu><button type="button" data-chat-auto-title><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg><span>Generate title</span></button><button type="button" data-chat-rename><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-edit"></use></svg><span>Rename</span></button><button type="button" class="danger-text" data-chat-delete><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-trash"></use></svg><span>Delete</span></button></div>
        </div>
      </header>
      <div class="chat-body" data-chat-messages></div>
      <button type="button" class="chat-scroll-bottom hidden" data-chat-scroll-bottom title="Scroll to bottom" aria-label="Scroll to bottom"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-down"></use></svg></button>
      <div class="chat-composer-shell">
        <form class="chat-composer" data-chat-form>
          <textarea data-chat-input rows="1" placeholder="Ask a question..." required></textarea>
          <div class="chat-composer-bar"><div class="chat-tools-wrap"><button type="button" class="chat-icon-button chat-tools-toggle" data-chat-tools-toggle aria-expanded="false" title="Tools" aria-label="Tools"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg></button><div class="chat-popover chat-tools-menu hidden" data-chat-tools-menu><label><input type="checkbox" data-chat-search checked><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg><span>Search</span></label><label><input type="checkbox" data-chat-read checked><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg><span>Read web</span></label><label><input type="checkbox" data-chat-process checked title="Process visibility only"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg><span>Process</span></label></div><div class="chat-tool-status" data-chat-tool-status></div></div><div class="chat-send-actions"><button type="button" class="chat-send-button hidden" data-chat-stop title="Stop" aria-label="Stop"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-stop"></use></svg></button><button type="submit" class="chat-send-button" data-chat-send title="Send" aria-label="Send"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-up"></use></svg></button></div></div>
        </form>
      </div>
      <div class="chat-settings-modal hidden" data-chat-settings-modal aria-hidden="true">
        <div class="chat-settings-backdrop" data-chat-settings-close></div>
        <form class="chat-settings-dialog" data-chat-settings-form role="dialog" aria-modal="true" aria-labelledby="account-chat-settings-title">
          <div class="chat-settings-head">
            <div>
              <h2 id="account-chat-settings-title">Chat settings</h2>
              <p>Saved to this conversation</p>
            </div>
            <button type="button" class="secondary action-icon" data-chat-settings-close title="Close" aria-label="Close settings"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg></button>
          </div>
          <div class="chat-settings-body">
            <section class="chat-settings-section"><h3 data-chat-model-section-title>Model and reasoning</h3><label class="chat-model-field">Model<select data-chat-model></select></label><fieldset class="chat-thinking chat-settings-thinking"><legend>Thinking</legend><label><input type="radio" name="account-chat-thinking" value="off">Off</label><label><input type="radio" name="account-chat-thinking" value="low">Low</label><label><input type="radio" name="account-chat-thinking" value="medium" checked>Medium</label><label><input type="radio" name="account-chat-thinking" value="high">High</label></fieldset><label class="chat-settings-field chat-max-tools-field">Max tool calls<input data-chat-max-tool-calls type="number" min="0" max="20" step="1" value="6" readonly></label></section>
            <section class="chat-settings-section"><h3 data-chat-instructions-title>Instructions</h3><details class="chat-default-system-prompt"><summary data-chat-default-system-title>Default system prompt</summary><span data-chat-default-system-hint>Always applied by TokenFlow. Your instructions below are appended.</span><pre data-chat-default-system-prompt></pre></details><label class="chat-settings-field">System prompt<textarea data-chat-system-prompt maxlength="8000" rows="7" placeholder="Optional instructions for this conversation"></textarea></label></section>
            <section class="chat-settings-section"><h3 data-chat-identity-title>Identity</h3><label class="chat-settings-field">My nickname<input data-chat-nickname maxlength="64" placeholder="Optional display name"></label><div class="chat-avatar-settings" aria-label="Avatar settings"><div class="chat-avatar-field"><div class="chat-avatar-card-head"><span>User avatar</span><input data-chat-user-avatar maxlength="16" placeholder="😀" aria-label="User avatar"></div><div class="chat-avatar-picker" aria-label="User avatar presets"></div></div><div class="chat-avatar-field"><div class="chat-avatar-card-head"><span>Assistant avatar</span><input data-chat-assistant-avatar maxlength="16" placeholder="🤖" aria-label="Assistant avatar"></div><div class="chat-avatar-picker" aria-label="Assistant avatar presets"></div></div></div></section>
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
