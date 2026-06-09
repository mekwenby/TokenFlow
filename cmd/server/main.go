package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

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

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	adminHandler.Register(r)

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
