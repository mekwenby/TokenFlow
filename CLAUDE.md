# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```sh
go run ./cmd/server                # development
go build -o tokenflow ./cmd/server # production binary
docker compose up --build -d       # containerized
```

Startup reads `GATEWAY_ADDR` (default `:8019`) and `GATEWAY_DATA_DIR` (default `data`).

## Test

```sh
go test ./...                       # all packages
go test ./internal/store/           # single package
go test -run TestName ./internal/... # specific test
```

Tests are standard-library only — no testify, ginkgo, etc. Each test creates its own SQLite via `t.TempDir()` and mocks upstream servers with `httptest.NewServer`.

## Architecture

**Entrypoint** (`cmd/server/main.go`): wires `config.Load()`, `secret.Load()` (AES-256-GCM box for provider API keys), `store.Open()` (SQLite), then mounts admin and proxy routes on a `chi/v5` router. Middleware stack: RequestID → RealIP → Logger → Recoverer.

**Store** (`internal/store/store.go`): single `*sql.DB` (modernc.org/sqlite, CGo-free). Manages admin users, providers (with encrypted API keys), model mappings, distribution keys, request logs, and usage stats. Auto-migrates schema on `Open()`. Provides `ResolveRoute(clientModel)` for the routing algorithm.

**Proxy** (`internal/proxy/proxy.go`): the gateway core. `Handler` holds the store, secret box, and an `*http.Client` (no timeout — streaming). Both `OpenAIChat` and `AnthropicMessages` delegate to a shared `handle()` method that:
1. Authenticates via distribution key (Bearer or x-api-key)
2. Resolves the route (mapping → provider model match → default fallback)
3. Decrypts the upstream API key
4. If cross-protocol (client OpenAI ↔ upstream Anthropic or vice versa), converts the request body via `internal/convert`
5. Proxies to upstream, converting SSE chunks on-the-fly if cross-protocol streaming
6. Logs the request (latency, tokens, status) — no prompt/response bodies stored

**Convert** (`internal/convert/convert.go`): pure functions for OpenAI↔Anthropic format conversion (requests, responses, SSE chunks). Handles tool calls, multimodal content, and cache token usage parsing. No side effects.

**Admin** (`internal/admin/admin.go`): embedded SPA served from `web/` via `embed.FS`. Login uses bcrypt-hashed passwords with 12h session cookies. CSRF token required on write endpoints (providers, mappings, keys). Reports i18n (en/zh) based on cookie or Accept-Language.

**Auth** (`internal/auth/auth.go`): SHA-256 key hashing with constant-time comparison, session cookie sign/verify via crypto/hmac.

**Secret** (`internal/secret/secret.go`): AES-256-GCM encrypt/decrypt for upstream API keys. Key material persisted in `data/app.secret`; auto-generated if missing.

## Routing algorithm

1. Exact model mapping match (client model → provider + upstream model)
2. If no mapping, find an enabled provider that lists the requested model
3. Fall back to the default provider's default model

## Key constraints

- Distribution keys shown once at creation; only the hash is stored
- Prompt and response bodies are **never** written to request logs
- `n > 1` (multiple completions) not supported — intentionally out of scope
- The `proxy_server.py` at the repo root is a separate auxiliary tool (FastAPI proxy for Anthropic→OpenAI), not part of the gateway itself
