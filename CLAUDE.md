# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```sh
go run ./cmd/server                # development
go build -o tokenflow ./cmd/server # production binary
docker compose up --build -d       # containerized (multi-stage, CGO_ENABLED=0)
```

Optional asset minification (requires Node.js):
```sh
npm install && npm run build        # minify JS/CSS into web/static/dist/
go generate ./web                   # same as above, via go:generate
```

Startup reads `GATEWAY_ADDR` (default `:8019`), `GATEWAY_DATA_DIR` (default `data`), and `INFOFLOW_BASE_URL` (default `https://infoflow.030399.xyz` — backend for chat web search / URL reading tools).

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
- `/admin*` — admin SPA (provider, mapping, key, user, log, stats, chat)
- `/account/*` — consumer self-service (register, login, dashboard, API key CRUD, chat)
- `POST /v1/chat/completions`, `GET /v1/models` — OpenAI-compatible proxy
- `POST /v1/messages`, `POST /anthropic/v1/messages`, `GET /anthropic/v1/models` — Anthropic-compatible proxy

**Config** (`internal/config/config.go`): loads `GATEWAY_ADDR`, `GATEWAY_DATA_DIR`, and `INFOFLOW_BASE_URL` from env vars with sensible defaults.

**Store** (`internal/store/store.go`): single `*sql.DB` (modernc.org/sqlite, CGo-free). Manages admin users, consumer users (with quota tracking), providers (with encrypted API keys), model mappings, distribution keys (admin-scoped and consumer-scoped), request logs, usage stats, and chat conversations/messages/settings. Auto-migrates schema on `Open()` including backward-compatible column additions. Provides `ResolveRoute(clientModel)` for the routing algorithm and `RecordRequest()` for transactional logging + stats + quota updates.

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

**Chat** (`internal/chat/`): built-in LLM chat with conversation management, SSE streaming, and tool use. `Service` wraps the store and secret box, reusing the routing algorithm (`ResolveRoute`) and usage recording. `SendMessage` emits per-chunk `delta` events via SSE and accumulates tool calls (web_search, read_url) that delegate to an external InfoFlow backend. Handles thinking-effort parameters with automatic retry fallbacks (remove thinking → remove streaming → non-streaming). Conversation titles are auto-generated via the same model-routing path. Routes are mounted under both `/admin/chat` and `/account/chat` via `RegisterRoutes` (`internal/chat/http.go`), which accepts a pluggable `RouteConfig` for owner resolution and CSRF enforcement. Has its own SSE parser (`internal/chat/sse.go`) separate from the proxy's.

**HTTP utilities** (`internal/httputil/`): shared helpers used by both admin and account — `WriteError`, `DecodePayload`, `IDParam`, `WriteResult`, `NormalizeLang`, `LanguageFromRequest`, `SetLanguageCookie`, `SafeNextPath`.

**Auth** (`internal/auth/auth.go`): SHA-256 key hashing with constant-time comparison, session cookie sign/verify via crypto/hmac. `NewSessions` is for admin (`/admin` path), `NewScopedSessions` for account (`/account` path) — same logic, different cookie names and paths to avoid collisions. Distribution key generation (`sk-` prefix, 32 random bytes).

**Secret** (`internal/secret/secret.go`): AES-256-GCM encrypt/decrypt for upstream API keys. Key material persisted in `data/app.secret`; auto-generated if missing.

**Web** (`web/web.go`): embeds `web/static/*` via `embed.FS` for serving CSS, JS, icons, and the logo. Frontend is organized as ES modules:

- `theme.js` — light/dark theme toggle (loads before paint on every page)
- `core/` — api.js (CSRF-aware fetch client factory), dom.js (esc, loading/error HTML, form busy), state.js (Store class via EventTarget), format.js, toast.js, confirm.js, nav.js
- `components/` — data-table.js (generic table rendering), crud-manager.js (form open/close/submit/delete patterns), chart.js (parameterized SVG bar/line charts)
- `admin/app.js` — main admin SPA entry point, dispatch table for `data-action` events
- `account/app.js` — consumer account portal, reuses core modules
- `chat/app.js` — chat UI (conversation sidebar, message streaming, tool call rendering)
- `css/` — tokens.css (design tokens + light/dark themes), base.css, components.css, charts.css, layout.css
- `dist/` — optional minified bundles produced by `npm run build`

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
