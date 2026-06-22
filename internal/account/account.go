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
	"tokenflow/internal/store"
)

const (
	sessionCookie   = "gateway_account_session"
	csrfCookie      = "gateway_account_csrf"
	adminLangCookie = "gateway_lang"
)

type Handler struct {
	store    *store.Store
	sessions *auth.Sessions
	tpl      *template.Template
}

type pageData struct {
	Title    string
	Email    string
	Error    string
	Message  string
	User     store.ConsumerUser
	Lang     string
	LangJSON template.JS
	I18NJSON template.JS
}

type keyPayload struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func New(st *store.Store) *Handler {
	return &Handler{
		store:    st,
		sessions: auth.NewScopedSessions(12*time.Hour, sessionCookie, csrfCookie, "/account"),
		tpl: template.Must(template.New("account").Funcs(template.FuncMap{
			"tr":              accountTr,
			"accountI18NJSON": accountI18NJSON,
			"compactNumber":   compactNumber,
			"jsonString":      jsonString,
		}).Parse(templates)),
	}
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
	r.Post("/account/logout", h.logout)
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Get("/account", h.dashboard)
		r.Get("/account/api/keys", h.keys)
		r.Post("/account/api/keys", h.keys)
		r.Put("/account/api/keys", h.keys)
		r.Delete("/account/api/keys", h.keys)
		r.Post("/account/api/keys/reset", h.resetKey)
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
	if err := h.sessions.Create(w, user.ID, user.Email); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w, r)
	http.Redirect(w, r, "/account/login", http.StatusSeeOther)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessions.Get(r)
	user, err := h.store.ConsumerUser(r.Context(), session.UserID)
	if err != nil || user.Status != store.ConsumerStatusEnabled {
		h.sessions.Clear(w, r)
		http.Redirect(w, r, "/account/login", http.StatusSeeOther)
		return
	}
	data := h.page(r, "title.dashboard")
	data.Email = session.Username
	data.User = user
	_ = h.tpl.ExecuteTemplate(w, "dashboard", data)
}

func (h *Handler) keys(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessions.Get(r)
	if !h.requireCSRFForWrite(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		keys, err := h.store.ConsumerDistributionKeys(r.Context(), session.UserID)
		h.writeResult(w, keys, err)
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
		h.writeResult(w, key, err)
	case http.MethodPut:
		var payload keyPayload
		if !decodePayload(w, r, &payload) {
			return
		}
		key, err := h.store.UpdateConsumerDistributionKey(r.Context(), session.UserID, payload.ID, strings.TrimSpace(payload.Name), payload.Enabled)
		h.writeResult(w, key, err)
	case http.MethodDelete:
		id := idParam(r)
		h.writeResult(w, map[string]bool{"ok": true}, h.store.DeleteConsumerDistributionKey(r.Context(), session.UserID, id))
	}
}

func (h *Handler) resetKey(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessions.Get(r)
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
	h.writeResult(w, key, err)
}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := h.sessions.Get(r)
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/account/api/") {
				h.writeLocalizedError(w, r, http.StatusUnauthorized, "login_required")
			} else {
				http.Redirect(w, r, "/account/login", http.StatusSeeOther)
			}
			return
		}
		user, err := h.store.ConsumerUser(r.Context(), session.UserID)
		if err != nil || user.Status != store.ConsumerStatusEnabled {
			h.sessions.Clear(w, r)
			if strings.HasPrefix(r.URL.Path, "/account/api/") {
				h.writeLocalizedError(w, r, http.StatusUnauthorized, "login_required")
			} else {
				http.Redirect(w, r, "/account/login", http.StatusSeeOther)
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
	if h.sessions.ValidateCSRF(r) {
		return true
	}
	h.writeLocalizedError(w, r, http.StatusForbidden, "csrf_invalid")
	return false
}

func (h *Handler) writeLocalizedError(w http.ResponseWriter, r *http.Request, status int, key string) {
	writeError(w, status, accountTr(accountLanguageFromRequest(r), key))
}

func (h *Handler) writeResult(w http.ResponseWriter, body any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func decodePayload(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

var accountTranslations = map[string]map[string]string{
	"en": {
		"app.title":              "TokenFlow",
		"title.register":         "Create account",
		"title.login":            "Account login",
		"title.dashboard":        "TokenFlow Account",
		"create_account":         "Create account",
		"account_login":          "Account login",
		"email":                  "Email",
		"password":               "Password",
		"register":               "Register",
		"login":                  "Log in",
		"logout":                 "Log out",
		"registration_submitted": "Registration submitted. Wait for an administrator to enable your account and assign quota.",
		"register_invalid":       "Use a valid email and a password with at least 8 characters.",
		"register_duplicate":     "This email is already registered.",
		"login_invalid":          "Invalid credentials or account is not enabled.",
		"usage":                  "Usage",
		"quota":                  "Quota",
		"used":                   "Used",
		"remaining":              "Remaining",
		"requests":               "Requests",
		"api_addresses":          "API addresses",
		"api_base":               "API base",
		"openai_chat":            "OpenAI chat",
		"openai_models":          "OpenAI models",
		"anthropic_messages":     "Anthropic messages",
		"anthropic_models":       "Anthropic models",
		"api_keys":               "API keys",
		"create_key":             "Create key",
		"name":                   "Name",
		"prefix":                 "Prefix",
		"status":                 "Status",
		"enabled":                "Enabled",
		"disabled":               "Disabled",
		"input_tokens":           "Input tokens",
		"cache_read_tokens":      "Cache read tokens",
		"output_tokens":          "Output tokens",
		"last_used":              "Last used",
		"edit":                   "Edit",
		"delete":                 "Delete",
		"regenerate":             "Regenerate",
		"save":                   "Save",
		"cancel":                 "Cancel",
		"new_key":                "New key",
		"empty":                  "No records yet.",
		"copy":                   "Copy",
		"copied":                 "Copied",
		"confirm_title":          "Confirm action",
		"delete_key_confirm":     "Delete this key?",
		"reset_key_confirm":      "Regenerate this key? The previous key stops working immediately.",
		"loading":                "Loading...",
		"saving":                 "Saving...",
		"saved":                  "Saved",
		"request_failed":         "Request failed",
		"login_required":         "login required",
		"csrf_invalid":           "invalid CSRF token",
		"api_key_default_name":   "API key",
	},
	"zh-CN": {
		"app.title":              "TokenFlow",
		"title.register":         "创建账号",
		"title.login":            "账号登录",
		"title.dashboard":        "TokenFlow 账号",
		"create_account":         "创建账号",
		"account_login":          "账号登录",
		"email":                  "邮箱",
		"password":               "密码",
		"register":               "注册",
		"login":                  "登录",
		"logout":                 "退出登录",
		"registration_submitted": "注册已提交，请等待管理员启用账号并分配额度。",
		"register_invalid":       "请输入有效邮箱，密码至少需要 8 个字符。",
		"register_duplicate":     "这个邮箱已经注册。",
		"login_invalid":          "登录信息错误，或账号尚未启用。",
		"usage":                  "使用情况",
		"quota":                  "额度",
		"used":                   "已用",
		"remaining":              "剩余",
		"requests":               "请求数",
		"api_addresses":          "API 地址",
		"api_base":               "API 基础地址",
		"openai_chat":            "OpenAI 对话",
		"openai_models":          "OpenAI 模型列表",
		"anthropic_messages":     "Anthropic 消息",
		"anthropic_models":       "Anthropic 模型列表",
		"api_keys":               "API Keys",
		"create_key":             "创建 Key",
		"name":                   "名称",
		"prefix":                 "前缀",
		"status":                 "状态",
		"enabled":                "已启用",
		"disabled":               "已禁用",
		"input_tokens":           "输入 Token",
		"cache_read_tokens":      "缓存命中 Token",
		"output_tokens":          "输出 Token",
		"last_used":              "最后使用",
		"edit":                   "编辑",
		"delete":                 "删除",
		"regenerate":             "重新生成",
		"save":                   "保存",
		"cancel":                 "取消",
		"new_key":                "新 Key",
		"empty":                  "暂无记录。",
		"copy":                   "复制",
		"copied":                 "已复制",
		"confirm_title":          "确认操作",
		"delete_key_confirm":     "确定删除这个 Key？",
		"reset_key_confirm":      "重新生成后旧 Key 会立即失效，确定继续？",
		"loading":                "加载中...",
		"saving":                 "保存中...",
		"saved":                  "已保存",
		"request_failed":         "请求失败",
		"login_required":         "需要先登录",
		"csrf_invalid":           "CSRF Token 无效",
		"api_key_default_name":   "API Key",
	},
}

func (h *Handler) page(r *http.Request, titleKey string) pageData {
	lang := accountLanguageFromRequest(r)
	return pageData{
		Title:    accountTr(lang, titleKey),
		Lang:     lang,
		LangJSON: jsonString(lang),
		I18NJSON: accountI18NJSON(lang),
	}
}

func accountLanguageFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(adminLangCookie); err == nil {
		if lang, ok := normalizeLang(cookie.Value); ok {
			return lang
		}
	}
	header := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(header, "zh") {
		return "zh-CN"
	}
	return "en"
}

func normalizeLang(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "en-us", "en-gb":
		return "en", true
	case "zh", "zh-cn", "zh_cn", "cn", "zh-hans":
		return "zh-CN", true
	default:
		return "", false
	}
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
{{define "auth_head"}}
<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><link rel="icon" href="/admin/static/tokenflow-logo.svg"><link rel="stylesheet" href="/admin/static/style.css"></head>
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
  <link rel="stylesheet" href="/admin/static/style.css">
</head>
<body class="admin-page">
  <header class="topbar">
    <div class="topbar-inner">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>TokenFlow</span></h1>
      <div class="top-actions">
        <span class="user-chip" title="{{.Email}}">{{.Email}}</span>
        <form method="post" action="/account/logout"><button type="submit" class="secondary icon-label"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-log-out"></use></svg><span>{{tr .Lang "logout"}}</span></button></form>
      </div>
    </div>
  </header>
  <main class="layout">
    <section class="panel stats-panel">
      <h2>{{tr .Lang "usage"}}</h2>
      <div class="stats-grid">
        <div class="stat"><span>{{tr .Lang "quota"}}</span><strong>{{compactNumber .User.QuotaTotalTokens}}</strong></div>
        <div class="stat"><span>{{tr .Lang "used"}}</span><strong>{{compactNumber .User.QuotaUsedTokens}}</strong></div>
        <div class="stat"><span>{{tr .Lang "remaining"}}</span><strong>{{compactNumber .User.QuotaRemainingTokens}}</strong></div>
        <div class="stat"><span>{{tr .Lang "requests"}}</span><strong>{{compactNumber .User.RequestCount}}</strong></div>
      </div>
    </section>
    <section class="panel">
      <h2>{{tr .Lang "api_addresses"}}</h2>
      <div class="api-grid" id="account-api-addresses"></div>
    </section>
    <section class="panel">
      <div class="section-head">
        <h2>{{tr .Lang "api_keys"}}</h2>
        <button type="button" class="icon-label" data-open-account-key><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg><span>{{tr .Lang "create_key"}}</span></button>
      </div>
      <form id="account-key-form" class="editor hidden">
        <input type="hidden" name="id">
        <label>{{tr .Lang "name"}}<input name="name" required></label>
        <label class="check account-key-enabled hidden"><input name="enabled" type="checkbox" checked> {{tr .Lang "enabled"}}</label>
        <div class="row-actions"><button type="submit">{{tr .Lang "save"}}</button><button type="button" class="secondary" data-cancel-account-key>{{tr .Lang "cancel"}}</button></div>
      </form>
      <p id="account-new-key" class="secret hidden"></p>
      <div id="account-keys" class="table-wrap"></div>
    </section>
  </main>
  <script>
    window.__ACCOUNT_LANG__ = {{.LangJSON}};
    window.__ACCOUNT_I18N__ = {{.I18NJSON}};
  </script>
  <script src="/admin/static/common.js"></script>
  <script src="/admin/static/account.js"></script>
</body>
</html>
{{end}}
`
