# TokenFlow

[中文](README.md) | English

TokenFlow is a Go 1.21 LLM gateway that exposes OpenAI-compatible and Anthropic-compatible public APIs. It supports SQLite persistence, encrypted upstream provider keys, model routing, SSE streaming conversion, request usage logging, and an embedded admin UI.

## Features

- OpenAI-compatible `POST /v1/chat/completions`.
- Anthropic-compatible `POST /v1/messages`.
- Legacy Anthropic path `POST /anthropic/v1/messages`.
- Model list endpoints for configured platform models.
- Cross-protocol request, response, and streaming conversion between OpenAI and Anthropic formats.
- Provider, model mapping, distribution key, request log, and usage-stat management through the admin UI.
- SQLite storage with automatic migrations.
- AES-256-GCM encryption for upstream provider API keys.

## Requirements

- Go 1.21 or newer.
- Docker and Docker Compose, if you want to run the containerized setup.

## Run Locally

```powershell
go run ./cmd/server
```

By default, the server listens on `http://localhost:8019`.

Open `http://localhost:8019/admin` and create the first admin user. Then configure:

1. A provider with protocol `openai` or `anthropic`.
2. One or more supported models for that provider. The default model is included automatically.
3. Optional model mappings from client-facing model names to a provider and upstream model.
4. A distribution key for API clients.

The service stores data in `data/gateway.db` and encrypts upstream API keys with `data/app.secret`.

## Build

```sh
go build -o tokenflow ./cmd/server
```

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

## Routing

TokenFlow resolves each request in this order:

1. Use an exact model mapping match from client model to provider and upstream model.
2. If no mapping exists, find an enabled provider that lists the requested model and use the same upstream model name.
3. If the model is unknown, fall back to the default provider and its default model.

## Configuration

Environment variables:

- `GATEWAY_ADDR`: listen address, default `:8019`.
- `GATEWAY_DATA_DIR`: data directory, default `data`.

## Security And Logging

- Distribution keys are only shown once when created. Only the hash is stored.
- Upstream provider API keys are encrypted before they are stored.
- Prompt and response bodies are not persisted in request logs.
- Request logs store operational metadata such as latency, token counts, status, provider, and model.

## Scope

The current gateway supports Chat Completions and Messages APIs. Responses API, Assistants, Batch, file uploads, audio, and full `n > 1` conversion are intentionally out of scope.
