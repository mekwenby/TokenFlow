# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```sh
go run ./cmd/server                # development
go build -o tokenflow ./cmd/server # production binary
docker compose up --build -d       # containerized (multi-stage, CGO_ENABLED=0)
```

Startup reads `GATEWAY_ADDR` (default `:8019`) and `GATEWAY_DATA_DIR` (default `data`).

## Test

```sh
go test ./...                       # all packages
go test ./internal/store/           # single package
go test -run TestName ./internal/... # specific test
```

Tests are standard-library only — no testify, ginkgo, etc. Each test creates its own SQLite via `t.TempDir()` and mocks upstream servers with `httptest.NewServer`. Only 3 direct dependencies: `chi/v5`, `golang.org/x/crypto`, `modernc.org/sqlite`.

## Architecture

**Entrypoint** (`cmd/server/main.go`): wires `config.Load()`, `secret.Load()` (AES-256-GCM box for provider API keys), `store.Open()` (SQLite), then mounts admin, account, and proxy routes on a `chi/v5` router. Middleware stack: RequestID → RealIP → Logger → Recoverer.

Routes:
- `GET /` — portal page (i18n en/zh) linking to both admin and account login
- `GET /healthz` — liveness check
- `/admin*` — admin SPA (provider, mapping, key, user, log, stats management)
- `/account/*` — consumer self-service (register, login, dashboard, API key CRUD)
- `POST /v1/chat/completions`, `GET /v1/models` — OpenAI-compatible proxy
- `POST /v1/messages`, `POST /anthropic/v1/messages`, `GET /anthropic/v1/models` — Anthropic-compatible proxy

**Store** (`internal/store/store.go`): single `*sql.DB` (modernc.org/sqlite, CGo-free). Manages admin users, consumer users (with quota tracking), providers (with encrypted API keys), model mappings, distribution keys (admin-scoped and consumer-scoped), request logs, and usage stats. Auto-migrates schema on `Open()` including backward-compatible column additions. Provides `ResolveRoute(clientModel)` for the routing algorithm and `RecordRequest()` for transactional logging + stats + quota updates.

**Proxy** (`internal/proxy/proxy.go`): the gateway core. `Handler` holds the store, secret box, and an `*http.Client` (no timeout — streaming). Both `OpenAIChat` and `AnthropicMessages` delegate to a shared `handle()` method that:
1. Authenticates via distribution key (Bearer or x-api-key)
2. Checks consumer quota if the key belongs to a consumer user
3. Resolves the route (mapping → provider model match → default fallback)
4. Decrypts the upstream API key
5. If cross-protocol (client OpenAI ↔ upstream Anthropic or vice versa), converts the request body via `internal/convert`
6. Proxies to upstream, converting SSE chunks on-the-fly if cross-protocol streaming
7. Logs the request and updates key/consumer stats via `RecordRequest` (latency, tokens, status) — no prompt/response bodies stored

**SSE** (`internal/proxy/sse.go`): low-level SSE parser (`readSSE`) that tokenizes a stream into `sseEvent{Event, Data}` structs. Used by the proxy to intercept and transform streaming chunks during cross-protocol proxying.

**Convert** (`internal/convert/convert.go`): pure functions for OpenAI↔Anthropic format conversion (requests, responses, SSE chunks). Handles tool calls, multimodal content, and cache token usage parsing. No side effects.

**Admin** (`internal/admin/admin.go`): embedded SPA served from `web/` via `embed.FS`. Login uses bcrypt-hashed passwords with 12h session cookies at path `/admin`. CSRF token required on write endpoints (providers, mappings, keys, users). i18n (en/zh) based on cookie or Accept-Language. Manages consumer users (status, quota) and model token detail reports.

**Account** (`internal/account/account.go`): consumer-facing self-service portal. Registration (pending by default, admin must enable), login, dashboard with quota/usage summary, and API key CRUD. Uses scoped sessions at path `/account` (separate cookies from admin), embedded Go templates with `text/template`, and i18n (en/zh-CN). Dashboard shows compact-number formatting (1.2K, 3.4M, 5B) for quota/usage.

**Auth** (`internal/auth/auth.go`): SHA-256 key hashing with constant-time comparison, session cookie sign/verify via crypto/hmac. `NewSessions` is for admin (`/admin` path), `NewScopedSessions` for account (`/account` path) — same logic, different cookie names and paths to avoid collisions. Distribution key generation (`sk-` prefix, 32 random bytes).

**Secret** (`internal/secret/secret.go`): AES-256-GCM encrypt/decrypt for upstream API keys. Key material persisted in `data/app.secret`; auto-generated if missing.

**Web** (`web/web.go`): embeds `web/static/*` via `embed.FS` for serving CSS, JS, icons, and the logo. Three JS bundles: `common.js` (shared utilities), `app.js` (admin SPA), `account.js` (account dashboard).

## Routing algorithm

1. Exact model mapping match (client model → provider + upstream model)
2. If no mapping, find an enabled provider that lists the requested model
3. Fall back to the default provider's default model

## Key constraints

- Distribution keys shown once at creation; only the hash is stored
- Prompt and response bodies are **never** written to request logs
- `n > 1` (multiple completions) not supported — intentionally out of scope
- Consumer users register as `pending`; an admin must enable them and assign quota
- The `proxy_server.py` at the repo root is a separate auxiliary tool (FastAPI proxy for Anthropic→OpenAI), not part of the gateway itself
