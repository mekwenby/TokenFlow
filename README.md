# 一念通流 TokenFlow

中文 | [English](README.en.md)

> 一念通流，百模可达。

一念通流 TokenFlow 是一个兼容 OpenAI 与 Anthropic 协议的 LLM 网关。它基于 Go 1.21，支持 SQLite 持久化、上游供应商密钥加密、模型路由、跨协议转换、SSE 流式代理、请求用量日志、管理员后台、普通用户自助门户，以及内置 LLM Chat。

## 界面预览

![一念通流 TokenFlow 管理后台概览](images/1.webp)

![一念通流 TokenFlow 模型 Token 明细](images/2.webp)

## 功能

- 兼容 OpenAI 的 `POST /v1/chat/completions`。
- 兼容 Anthropic 的 `POST /v1/messages`。
- 兼容旧版 Anthropic 路径 `POST /anthropic/v1/messages`。
- 为客户端返回已配置平台模型的模型列表接口。
- 支持 OpenAI 与 Anthropic 格式之间的跨协议请求、响应和流式转换。
- 首页门户区分管理员入口与普通用户入口。
- 管理员后台可维护供应商、模型映射、分发密钥、普通用户、请求日志、Token 用量统计和模型 Token 明细。
- 普通用户可自助注册、登录、查看额度、管理自己的 API Key，并查看自己的请求记录。
- 管理员和普通用户都可以使用内置 LLM Chat；聊天支持会话、模型选择、思考强度、自定义系统提示词、昵称，以及可选的网页搜索和 URL 读取工具。
- 普通用户和管理员门户都支持安装为 Android PWA，分别默认打开 `/account/chat` 和 `/admin/chat`。
- 提供完全本地运行的原生 Android Chat App，由用户自行配置 OpenAI Chat Completions、OpenAI Responses 或 Anthropic Messages 供应商；支持本地工作区、附件与视觉兜底、网页工具和 MiMo TTS，会话仅保存在设备上。
- 使用 SQLite 存储，并在启动时自动迁移数据库结构。
- 使用 AES-256-GCM 加密保存上游供应商 API 密钥。

## 环境要求

- Go 1.21 或更高版本。
- Docker 和 Docker Compose（仅容器化运行时需要）。
- Node.js（仅在需要重新构建前端压缩资源时需要）。

## 本地运行

```powershell
go run ./cmd/server
```

默认情况下，服务监听 `http://localhost:8019`。

常用入口：

- `GET /`：首页门户，按语言展示管理员和普通用户入口。
- `GET /healthz`：健康检查，正常返回 `{"ok":true}`。
- `GET /admin`：管理员后台。首次运行会引导创建第一个管理员用户。
- `GET /admin/chat`：管理员 LLM Chat。
- `GET /account/register`：普通用户注册。新用户默认为 `pending`。
- `GET /account/login`：普通用户登录。
- `GET /account`：普通用户控制台。
- `GET /account/chat`：普通用户 LLM Chat。

首次启动后的建议配置流程：

1. 一个协议为 `openai` 或 `anthropic` 的供应商。
2. 该供应商支持的一个或多个模型。默认模型会自动包含在内。
3. 可选的模型映射，将客户端模型名映射到指定供应商和上游模型。
4. 给 API 客户端使用的管理员分发密钥。
5. 审批普通用户，将状态改为 `enabled`，并分配 Token 额度。
6. 让普通用户在自己的控制台创建 API Key。

服务会把数据保存到 `data/gateway.db`，并使用 `data/app.secret` 加密上游 API 密钥。

## Android PWA

普通用户和管理员都可通过 Android Chrome 的浏览器菜单安装 TokenFlow，无需页面内安装按钮。普通用户应用以 standalone 模式打开 `/account/chat`，管理员应用打开 `/admin/chat`。

生产环境必须使用 HTTPS 才能注册 Service Worker 和安装 PWA；`localhost` 仅适用于本地开发验证。离线模式只显示公开离线提示页，不支持离线聊天，也不会缓存登录页、账户页、聊天内容、认证页面或任何 API 响应。

## 原生 Android App

原生 Kotlin + Jetpack Compose 工程位于 `android/`，应用名为“一念通流”，当前版本为 2.3.3（`versionCode 8`），支持 Android 8.0（API 26）及以上版本。App 不需要登录，不访问 TokenFlow 移动接口；供应商、模型、会话、消息、收藏、笔记、智能体和知识库均由 App 在本地管理。Release 继续使用外部 Android keystore 签名，不需要 `TOKENFLOW_BASE_URL`。

当前版本支持三种模型协议的多轮流式聊天、会话分支/置顶/归档、图片和文档附件、系统相机 JPEG 75 压缩、模型视觉检测与兜底、Exa 搜索、InfoFlow/内置 URL 读取、Markdown/GFM 与安全内联 HTML、代码高亮，以及 MiMo 语音生成与 Media3 播放。语音自动播放事件只对发起生成时的会话页面实例有效，从笔记、收藏等页面返回不会重放旧语音。

```powershell
cd android
.\gradlew.bat testDebugUnitTest lintDebug assembleDebug assembleDebugAndroidTest
```

详细的供应商配置、Release 签名、加密导入导出和设备测试说明见 [`android/README.md`](android/README.md)。

维护现有 Go 后端/PWA 时，生产部署使用的 SSH 私钥路径为 `~/.ssh/LotusSSL`（Windows 通常为 `C:\Users\<用户名>\.ssh\LotusSSL`）。该私钥与 Android APK 签名 keystore 是两类不同凭据，均不得提交到仓库。

## 构建

```sh
go build -o tokenflow ./cmd/server
```

可选的前端资源压缩构建：

```sh
npm install
npm run build
```

也可以通过 Go 生成命令触发同样的构建：

```sh
go generate ./web
```

构建产物会写入 `web/static/dist/`，用于生成压缩后的 JS/CSS 文件和 manifest。

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

健康检查：

```sh
curl http://localhost:8019/healthz
```

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

移动客户端接口：

- `POST /mobile/v1/session`：使用普通用户邮箱、密码和设备名登录，仅本次响应返回 `tfm_` Bearer Token。
- `GET /mobile/v1/session`：获取当前普通用户和额度摘要，并按规则滚动续期。
- `DELETE /mobile/v1/session`：只撤销当前设备令牌。
- `/mobile/v1/chat/*`：Bearer 保护的模型、会话、消息、停止、重新生成和标题接口。

移动令牌有效期为滚动 30 天，剩余 7 天内访问时续期；数据库只保存 SHA-256 哈希。账号被禁用或删除时，其全部移动令牌都会失效。

## 架构

- `cmd/server/main.go`：加载配置、密钥盒和 SQLite store，注册首页、健康检查、管理员、普通用户、聊天和代理路由。
- `internal/store`：单个 `*sql.DB`，负责管理员、普通用户、供应商、模型映射、分发密钥、请求日志、用量统计、聊天会话和消息；启动时自动迁移 schema。
- `internal/proxy`：网关核心，负责分发密钥鉴权、消费者额度检查、路由解析、上游密钥解密、跨协议转换、SSE 转换和请求记录。
- `internal/convert`：纯函数实现 OpenAI 与 Anthropic 请求、响应和 SSE chunk 的相互转换。
- `internal/admin`：管理员后台，包含供应商、模型映射、Key、用户、日志、统计和聊天页面。
- `internal/account`：普通用户自助门户，包含注册、登录、额度概览、API Key 管理、请求日志和聊天页面。
- `internal/chat`：内置聊天服务，复用模型路由与用量记录，支持网页搜索和 URL 读取工具。
- `internal/mobile`：原生移动客户端的 Bearer 会话和 Chat 路由，不改变网页 Cookie/CSRF 认证。
- `internal/httputil`：管理员和普通用户模块共用的 HTTP、语言、错误响应和安全跳转工具。
- `web/static`：嵌入式前端资源，按 `core/`、`components/`、`admin/`、`account/`、`chat/`、`css/` 和可选 `dist/` 组织。

## 路由规则

一念通流 TokenFlow 会按以下顺序解析每个请求：

1. 优先使用精确模型映射，将客户端模型映射到指定供应商和上游模型。
2. 如果没有映射，则查找已启用且声明支持该模型的供应商，并使用相同的上游模型名。
3. 如果模型未知，则回退到默认供应商和默认模型。

## 配置

环境变量：

- `GATEWAY_ADDR`：监听地址，默认值为 `:8019`。
- `GATEWAY_DATA_DIR`：数据目录，默认值为 `data`。
- `INFOFLOW_BASE_URL`：内置聊天网页搜索和 URL 读取工具使用的 InfoFlow 服务地址，默认值为 `https://infoflow.030399.xyz`。
- `CHAT_CONTEXT_MAX_RUNES`：内置聊天发送给上游前的上下文字符预算，默认值为 `262144`；单条用户消息固定最多 `131072` 个 Unicode 字符。

## 安全与日志

- 分发密钥只会在创建时显示一次，数据库中只保存哈希值。
- 上游供应商 API 密钥会加密后再保存。
- 请求日志不会持久化保存 prompt 或 response 正文。
- 请求日志只记录延迟、token 数量、状态码、供应商、模型、Key、用户等运行元数据。
- 普通用户注册后默认为 `pending`，必须由管理员启用并分配额度后才能使用 API Key 或聊天。
- 管理员和普通用户使用不同路径和 cookie 作用域的会话，避免登录态互相覆盖。

## 范围

当前网关支持 Chat Completions 和 Messages APIs。Responses API、Assistants、Batch、文件上传、音频，以及完整的 `n > 1` 转换暂不支持，属于刻意不覆盖的范围。
