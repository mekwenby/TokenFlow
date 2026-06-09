# TokenFlow

中文 | [English](README.en.md)

TokenFlow 是一个基于 Go 1.21 的 LLM 网关，对外提供兼容 OpenAI 和 Anthropic 的 API。它支持 SQLite 持久化、上游供应商密钥加密、模型路由、SSE 流式转换、请求用量日志，以及内嵌管理后台。

## 功能

- 兼容 OpenAI 的 `POST /v1/chat/completions`。
- 兼容 Anthropic 的 `POST /v1/messages`。
- 兼容旧版 Anthropic 路径 `POST /anthropic/v1/messages`。
- 为客户端返回已配置平台模型的模型列表接口。
- 支持 OpenAI 与 Anthropic 格式之间的跨协议请求、响应和流式转换。
- 可通过管理后台维护供应商、模型映射、分发密钥、请求日志和用量统计。
- 使用 SQLite 存储，并在启动时自动迁移数据库结构。
- 使用 AES-256-GCM 加密保存上游供应商 API 密钥。

## 环境要求

- Go 1.21 或更高版本。
- 如果需要容器化运行，需要 Docker 和 Docker Compose。

## 本地运行

```powershell
go run ./cmd/server
```

默认情况下，服务监听 `http://localhost:8019`。

打开 `http://localhost:8019/admin` 并创建第一个管理员用户。然后配置：

1. 一个协议为 `openai` 或 `anthropic` 的供应商。
2. 该供应商支持的一个或多个模型。默认模型会自动包含在内。
3. 可选的模型映射，将客户端模型名映射到指定供应商和上游模型。
4. 给 API 客户端使用的分发密钥。

服务会把数据保存到 `data/gateway.db`，并使用 `data/app.secret` 加密上游 API 密钥。

## 构建

```sh
go build -o tokenflow ./cmd/server
```

## 测试

```sh
go test ./...
```

运行单个包或指定测试：

```sh
go test ./internal/store/
go test -run TestName ./internal/...
```

## Docker

```sh
docker compose up --build -d
```

容器会暴露 `8019` 端口，并将数据持久化到 `./data`。

## 对外 API

兼容 OpenAI 的 Chat Completions：

```sh
curl http://localhost:8019/v1/chat/completions \
  -H "Authorization: Bearer sk-your-distribution-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"client-model","messages":[{"role":"user","content":"hello"}]}'
```

兼容 Anthropic 的 Messages：

```sh
curl http://localhost:8019/v1/messages \
  -H "x-api-key: sk-your-distribution-key" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"client-model","max_tokens":256,"messages":[{"role":"user","content":"hello"}]}'
```

同时支持旧版路径 `POST /anthropic/v1/messages`。

模型列表接口：

- `GET /v1/models`
- `GET /anthropic/v1/models`

## 路由规则

TokenFlow 会按以下顺序解析每个请求：

1. 优先使用精确模型映射，将客户端模型映射到指定供应商和上游模型。
2. 如果没有映射，则查找已启用且声明支持该模型的供应商，并使用相同的上游模型名。
3. 如果模型未知，则回退到默认供应商和默认模型。

## 配置

环境变量：

- `GATEWAY_ADDR`：监听地址，默认值为 `:8019`。
- `GATEWAY_DATA_DIR`：数据目录，默认值为 `data`。

## 安全与日志

- 分发密钥只会在创建时显示一次，数据库中只保存哈希值。
- 上游供应商 API 密钥会加密后再保存。
- 请求日志不会持久化保存 prompt 或 response 正文。
- 请求日志只记录延迟、token 数量、状态码、供应商、模型等运行元数据。

## 范围

当前网关支持 Chat Completions 和 Messages APIs。Responses API、Assistants、Batch、文件上传、音频，以及完整的 `n > 1` 转换暂不支持，属于刻意不覆盖的范围。
