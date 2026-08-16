# TokenFlow

[中文](README.md) | English

TokenFlow is a Go 1.21 LLM gateway that exposes OpenAI-compatible and Anthropic-compatible public APIs. It supports SQLite persistence, encrypted upstream provider keys, model routing, cross-protocol conversion, SSE proxying, request usage logging, an embedded admin UI, a consumer self-service portal, and built-in LLM Chat.

## Screenshots

![TokenFlow admin dashboard](images/1.webp)

![TokenFlow model token details](images/2.webp)

## Features

- OpenAI-compatible `POST /v1/chat/completions`.
- Anthropic-compatible `POST /v1/messages`.
- Legacy Anthropic path `POST /anthropic/v1/messages`.
- Model list endpoints for configured platform models.
- Cross-protocol request, response, and streaming conversion between OpenAI and Anthropic formats.
- Portal page with separate admin and consumer entry points.
- Admin UI for providers, model mappings, distribution keys, consumer users, request logs, token usage reports, and model token details.
- Consumer self-service for registration, login, quota review, personal API key management, and personal request logs.
- Built-in LLM Chat for both admins and consumers, with conversations, model selection, thinking effort, custom system prompts, nicknames, and optional web search / URL reading tools.
- Installable Android PWAs for consumer and admin portals, opening `/account/chat` and `/admin/chat` respectively.
- A fully local native Android Chat app where users configure OpenAI Chat Completions, OpenAI Responses, or Anthropic Messages providers; it includes a local workspace, attachments and vision fallback, web tools, and MiMo TTS, while conversations stay on the device.
- SQLite storage with automatic migrations.
- AES-256-GCM encryption for upstream provider API keys.

## Requirements

- Go 1.21 or newer.
- Docker and Docker Compose, if you want to run the containerized setup.
- Node.js, only if you need to rebuild minified frontend assets.

## Run Locally

```powershell
go run ./cmd/server
```

By default, the server listens on `http://localhost:8019`.

Common entry points:

- `GET /`: portal page with localized admin and consumer links.
- `GET /healthz`: liveness check, returns `{"ok":true}` when healthy.
- `GET /admin`: admin dashboard. The first run redirects to admin setup.
- `GET /admin/chat`: admin LLM Chat.
- `GET /account/register`: consumer registration. New users start as `pending`.
- `GET /account/login`: consumer login.
- `GET /account`: consumer dashboard.
- `GET /account/chat`: consumer LLM Chat.

Recommended first-run setup:

1. A provider with protocol `openai` or `anthropic`.
2. One or more supported models for that provider. The default model is included automatically.
3. Optional model mappings from client-facing model names to a provider and upstream model.
4. An admin-owned distribution key for API clients.
5. Approve consumer users by setting them to `enabled` and assigning token quota.
6. Let consumers create their own API keys from the consumer dashboard.

The service stores data in `data/gateway.db` and encrypts upstream API keys with `data/app.secret`.

## Android PWA

Consumers and administrators can install TokenFlow from the Android Chrome browser menu; there is no in-page install button. The consumer standalone application opens `/account/chat`, while the admin application opens `/admin/chat`.

Production deployments must use HTTPS for service worker registration and PWA installation. `localhost` is supported only for local development checks. Offline mode only shows a public offline page: offline chat is not supported, and login pages, account pages, chat content, authenticated HTML, and API responses are never cached.

## Native Android App

The native Kotlin and Jetpack Compose project lives in `android/`. Version 2.3.3 (`versionCode 8`) supports Android 8.0 (API 26) and newer. It has no login and does not call TokenFlow mobile endpoints; providers, models, conversations, messages, bookmarks, notes, agents, and knowledge files are managed locally. Release builds still use the external Android signing keystore and do not require `TOKENFLOW_BASE_URL`.

The current version supports multi-turn streaming across all three model protocols, conversation branching/pinning/archiving, image and document attachments, JPEG 75 system-camera capture, model vision tests and fallback, Exa search, InfoFlow/built-in URL reading, Markdown/GFM with safe inline HTML, syntax highlighting, and MiMo speech generation with Media3 playback. Speech auto-play events are scoped to the chat page instance that requested generation, so returning from notes, bookmarks, or another workspace page does not replay old audio.

```powershell
cd android
.\gradlew.bat testDebugUnitTest lintDebug assembleDebug assembleDebugAndroidTest
```

See [`android/README.md`](android/README.md) for provider setup, encrypted configuration transfer, release signing, APK paths, and device test requirements.

Production maintenance of the existing Go backend/PWA uses the SSH private key at `~/.ssh/LotusSSL` (typically `C:\Users\<username>\.ssh\LotusSSL` on Windows). This SSH key is separate from the Android APK signing keystore; neither credential belongs in the repository.

## Build

```sh
go build -o tokenflow ./cmd/server
```

Optional frontend asset minification:

```sh
npm install
npm run build
```

You can also trigger the same build through Go generate:

```sh
go generate ./web
```

The generated assets are written to `web/static/dist/`, including minified JS/CSS files and a manifest.

## Test

```sh
go test ./...
```

Run a single package or test:

```sh
go test ./internal/store/
go test -run TestName ./internal/...
```

## Docker

```sh
docker compose up --build -d
```

The container exposes port `8019` and persists data in `./data`.

## Public APIs

Health check:

```sh
curl http://localhost:8019/healthz
```

OpenAI-compatible chat completions:

```sh
curl http://localhost:8019/v1/chat/completions \
  -H "Authorization: Bearer sk-your-distribution-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"client-model","messages":[{"role":"user","content":"hello"}]}'
```

Anthropic-compatible messages:

```sh
curl http://localhost:8019/v1/messages \
  -H "x-api-key: sk-your-distribution-key" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"client-model","max_tokens":256,"messages":[{"role":"user","content":"hello"}]}'
```

The legacy path `POST /anthropic/v1/messages` is also supported.

Model list endpoints:

- `GET /v1/models`
- `GET /anthropic/v1/models`

Mobile client endpoints:

- `POST /mobile/v1/session`: signs in a consumer with email, password, and device name; the `tfm_` Bearer token is returned only once.
- `GET /mobile/v1/session`: returns the current consumer and quota summary and applies rolling renewal.
- `DELETE /mobile/v1/session`: revokes only the current device token.
- `/mobile/v1/chat/*`: Bearer-protected model, conversation, message, stop, regenerate, and title routes.

Mobile tokens have a rolling 30-day lifetime and renew when less than seven days remain. Only SHA-256 token hashes are stored. Disabling or deleting a consumer revokes all mobile sessions.

## Architecture

- `cmd/server/main.go`: loads configuration, the secret box, and the SQLite store, then registers the portal, health check, admin, account, chat, and proxy routes.
- `internal/store`: single `*sql.DB` for admin users, consumer users, providers, model mappings, distribution keys, request logs, usage reports, chat conversations, and chat messages. It auto-migrates schema on startup.
- `internal/proxy`: gateway core for distribution-key authentication, consumer quota checks, route resolution, upstream key decryption, cross-protocol conversion, SSE conversion, and request logging.
- `internal/convert`: pure OpenAI / Anthropic request, response, and SSE chunk conversion functions.
- `internal/admin`: admin UI for providers, model mappings, keys, users, logs, reports, and chat.
- `internal/account`: consumer self-service portal for registration, login, quota overview, API key management, request logs, and chat.
- `internal/chat`: built-in chat service that reuses model routing and usage logging, with optional web search and URL reading tools.
- `internal/mobile`: Bearer sessions and Chat routes for the native app, without changing web cookie or CSRF authentication.
- `internal/httputil`: shared HTTP, language, error response, and safe redirect helpers used by admin and account handlers.
- `web/static`: embedded frontend assets organized into `core/`, `components/`, `admin/`, `account/`, `chat/`, `css/`, and optional `dist/`.

## Routing

TokenFlow resolves each request in this order:

1. Use an exact model mapping match from client model to provider and upstream model.
2. If no mapping exists, find an enabled provider that lists the requested model and use the same upstream model name.
3. If the model is unknown, fall back to the default provider and its default model.

## Configuration

Environment variables:

- `GATEWAY_ADDR`: listen address, default `:8019`.
- `GATEWAY_DATA_DIR`: data directory, default `data`.
- `INFOFLOW_BASE_URL`: InfoFlow service base URL used by the built-in chat web search and URL reading tools, default `https://infoflow.030399.xyz`.
- `CHAT_CONTEXT_MAX_RUNES`: character budget applied before built-in Chat sends context upstream, default `262144`; each user message is limited to `131072` Unicode characters.

## Security And Logging

- Distribution keys are only shown once when created. Only the hash is stored.
- Upstream provider API keys are encrypted before they are stored.
- Prompt and response bodies are not persisted in request logs.
- Request logs store operational metadata such as latency, token counts, status, provider, model, key, and user.
- Consumer registrations start as `pending`; an admin must enable the user and assign quota before the user can use API keys or chat.
- Admin and consumer sessions use different paths and cookie scopes, so the two login states do not overwrite each other.

## Scope

The current gateway supports Chat Completions and Messages APIs. Responses API, Assistants, Batch, file uploads, audio, and full `n > 1` conversion are intentionally out of scope.
