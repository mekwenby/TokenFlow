package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"tokenflow/internal/account"
	"tokenflow/internal/admin"
	"tokenflow/internal/config"
	"tokenflow/internal/proxy"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	box, err := secret.Load(cfg.SecretPath)
	if err != nil {
		log.Fatalf("load app secret: %v", err)
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer st.Close()

	proxyHandler := proxy.New(st, box)
	adminHandler := admin.New(st, box)
	accountHandler := account.New(st)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", home)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	adminHandler.Register(r)
	accountHandler.Register(r)

	r.Post("/v1/chat/completions", proxyHandler.OpenAIChat)
	r.Get("/v1/models", proxyHandler.OpenAIModels)
	r.Post("/v1/messages", proxyHandler.AnthropicMessages)
	r.Post("/anthropic/v1/messages", proxyHandler.AnthropicMessages)
	r.Get("/anthropic/v1/models", proxyHandler.AnthropicModels)

	log.Printf("TokenFlow listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, r); err != nil {
		log.Fatal(err)
	}
}

type homeData struct {
	Lang          string
	Title         string
	UserLogin     string
	UserRegister  string
	AdminLogin    string
	UserTitle     string
	AdminTitle    string
	UserSubtitle  string
	AdminSubtitle string
}

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="icon" href="/admin/static/tokenflow-logo.svg">
  <link rel="stylesheet" href="/admin/static/style.css">
</head>
<body class="auth-page">
  <main class="auth-card portal-card">
    <div class="auth-head">
      <h1 class="brand"><img src="/admin/static/tokenflow-logo.svg" alt="" aria-hidden="true"><span>TokenFlow</span></h1>
    </div>
    <div class="portal-grid">
      <section class="portal-option">
        <h2>{{.UserTitle}}</h2>
        <p>{{.UserSubtitle}}</p>
        <div class="portal-actions">
          <a class="button-link" href="/account/login">{{.UserLogin}}</a>
          <a class="button-link secondary" href="/account/register">{{.UserRegister}}</a>
        </div>
      </section>
      <section class="portal-option">
        <h2>{{.AdminTitle}}</h2>
        <p>{{.AdminSubtitle}}</p>
        <div class="portal-actions">
          <a class="button-link" href="/admin">{{.AdminLogin}}</a>
        </div>
      </section>
    </div>
  </main>
</body>
</html>`))

func home(w http.ResponseWriter, r *http.Request) {
	data := homeText(homeLanguage(r))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = homeTemplate.Execute(w, data)
}

func homeLanguage(r *http.Request) string {
	header := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(header, "zh") {
		return "zh-CN"
	}
	return "en"
}

func homeText(lang string) homeData {
	if lang == "zh-CN" {
		return homeData{
			Lang:          "zh-CN",
			Title:         "TokenFlow",
			UserLogin:     "普通用户登录",
			UserRegister:  "注册普通用户",
			AdminLogin:    "管理员登录",
			UserTitle:     "普通用户",
			AdminTitle:    "管理员",
			UserSubtitle:  "查看额度并管理自己的 API Key。",
			AdminSubtitle: "管理供应商、模型映射、分发 Key 和用户。",
		}
	}
	return homeData{
		Lang:          "en",
		Title:         "TokenFlow",
		UserLogin:     "User login",
		UserRegister:  "Register",
		AdminLogin:    "Admin login",
		UserTitle:     "User",
		AdminTitle:    "Admin",
		UserSubtitle:  "View quota and manage your API keys.",
		AdminSubtitle: "Manage providers, model mappings, keys, and users.",
	}
}
