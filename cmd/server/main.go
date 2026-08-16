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
	"tokenflow/internal/chat"
	"tokenflow/internal/config"
	"tokenflow/internal/mobile"
	"tokenflow/internal/proxy"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
	"tokenflow/web"
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
	chatService := chat.NewService(st, box, cfg.InfoFlowBaseURL, cfg.ChatContextMaxRunes)
	adminHandler.SetChatService(chatService)
	accountHandler.SetChatService(chatService)
	mobileHandler := mobile.New(st, chatService)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", home)
	r.Get("/manifest.webmanifest", web.Manifest)
	r.Get("/admin/manifest.webmanifest", web.AdminManifest)
	r.Get("/service-worker.js", web.ServiceWorker)
	r.Get("/offline", web.Offline)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	adminHandler.Register(r)
	accountHandler.Register(r)
	mobileHandler.Register(r)

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
	Lang                string
	Title               string
	BrandName           string
	FullBrand           string
	EnglishBrand        string
	GatewayLabel        string
	AdminLogin          string
	Kicker              string
	Headline            string
	Description         string
	UserLogin           string
	UserRegister        string
	RouteTitle          string
	RouteSubtitle       string
	RequestLabel        string
	GatewayCore         string
	UpstreamLabel       string
	AuthLabel           string
	RouteLabel          string
	ConvertLabel        string
	StreamLabel         string
	OpenAIUpstream      string
	AnthropicUpstream   string
	CapabilitiesLabel   string
	EncryptedKeys       string
	CrossProtocol       string
	StreamingResponses  string
	ChatKicker          string
	ChatTitle           string
	ChatDescription     string
	ChatConversations   string
	ChatModelChoice     string
	ChatWebTools        string
	ChatRichOutput      string
	ChatPreviewLabel    string
	ChatNewConversation string
	ChatPrompt          string
	ChatRouteStatus     string
	ChatComposer        string
	ChatSearch          string
	ChatRead            string
	ChatProcess         string
}

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="icon" type="image/png" href="/admin/static/tokenflow-logo.png">
  <link rel="stylesheet" href="/admin/static/css/tokens.css">
  <link rel="stylesheet" href="/admin/static/css/base.css">
  <link rel="stylesheet" href="/admin/static/css/components.css">
  <link rel="stylesheet" href="/admin/static/css/layout.css">
  <link rel="stylesheet" href="/admin/static/css/home.css">
  <script src="/admin/static/theme.js"></script>
</head>
<body class="home-page">
  <header class="home-header">
    <div class="home-header-inner">
      <a class="home-brand" href="/" aria-label="{{.FullBrand}}">
        <img src="/admin/static/tokenflow-logo.png" alt="" aria-hidden="true">
        <span><strong>{{.BrandName}}</strong><small>{{.GatewayLabel}}</small></span>
      </a>
      <a class="home-admin-link" href="/admin">
        <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-settings"></use></svg>
        <span>{{.AdminLogin}}</span>
        <svg class="icon home-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg>
      </a>
    </div>
  </header>

  <main class="home-main">
    <section class="home-intro" aria-labelledby="home-title">
      <p class="home-kicker">
        <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg>
        <span>{{.Kicker}}</span>
      </p>
      <h1 id="home-title">{{.BrandName}}</h1>
      {{if .EnglishBrand}}<p class="home-english-brand">{{.EnglishBrand}}</p>{{end}}
      <p class="home-headline">{{.Headline}}</p>
      <p class="home-description">{{.Description}}</p>
      <div class="home-actions">
        <a class="button-link home-primary-action" href="/account/login">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-users"></use></svg>
          <span>{{.UserLogin}}</span>
          <svg class="icon home-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg>
        </a>
        <a class="button-link secondary home-secondary-action" href="/account/register">
          <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-add"></use></svg>
          <span>{{.UserRegister}}</span>
        </a>
      </div>
      <div class="home-capabilities" aria-label="{{.CapabilitiesLabel}}">
        <span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg>{{.EncryptedKeys}}</span>
        <span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg>{{.CrossProtocol}}</span>
        <span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-up"></use></svg>{{.StreamingResponses}}</span>
      </div>
    </section>

    <section class="route-console" data-home-motion aria-labelledby="route-console-title">
      <div class="route-console-head">
        <div class="route-console-title">
          <span class="route-console-icon"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg></span>
          <div>
            <h2 id="route-console-title">{{.RouteTitle}}</h2>
            <p>{{.RouteSubtitle}}</p>
          </div>
        </div>
        <span class="route-console-protocol">OpenAI + Anthropic</span>
      </div>

      <div class="route-map">
        <div class="route-column route-requests">
          <h3>{{.RequestLabel}}</h3>
          <span class="route-flow-highlight route-request-highlight route-flow-animated" aria-hidden="true"></span>
          <div class="route-node">
            <span>OpenAI API</span>
            <code>/v1/chat/completions</code>
          </div>
          <div class="route-node">
            <span>Anthropic API</span>
            <code>/v1/messages</code>
          </div>
        </div>

        <div class="route-connector route-connector-inbound" aria-hidden="true">
          <span class="route-packet route-flow-animated"></span>
        </div>

        <div class="route-core">
          <div class="route-core-brand">
            <img src="/admin/static/tokenflow-logo.png" alt="" aria-hidden="true">
            <span><strong>{{.BrandName}}</strong><small>{{if .EnglishBrand}}{{.EnglishBrand}} · {{end}}{{.GatewayCore}}</small></span>
          </div>
          <ol class="route-steps">
            <li><span class="route-progress route-flow-animated" aria-hidden="true"></span><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-key"></use></svg><span>{{.AuthLabel}}</span></li>
            <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-route"></use></svg><span>{{.RouteLabel}}</span></li>
            <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg><span>{{.ConvertLabel}}</span></li>
            <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-arrow-up"></use></svg><span>{{.StreamLabel}}</span></li>
          </ol>
        </div>

        <div class="route-connector route-connector-outbound" aria-hidden="true">
          <span class="route-packet route-flow-animated"></span>
        </div>

        <div class="route-column route-upstreams">
          <h3>{{.UpstreamLabel}}</h3>
          <span class="route-flow-highlight route-upstream-highlight route-flow-animated" aria-hidden="true"></span>
          <div class="route-node route-upstream-node">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-server"></use></svg>
            <span><strong>{{.OpenAIUpstream}}</strong><small>OpenAI</small></span>
          </div>
          <div class="route-node route-upstream-node">
            <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-server"></use></svg>
            <span><strong>{{.AnthropicUpstream}}</strong><small>Anthropic</small></span>
          </div>
        </div>
      </div>
    </section>

    <section class="home-chat-showcase" data-home-motion aria-labelledby="home-chat-title">
      <div class="home-chat-copy">
        <p class="home-chat-kicker"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg>{{.ChatKicker}}</p>
        <h2 id="home-chat-title">{{.ChatTitle}}</h2>
        <p>{{.ChatDescription}}</p>
        <ul>
          <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-list"></use></svg>{{.ChatConversations}}</li>
          <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg>{{.ChatModelChoice}}</li>
          <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-search"></use></svg>{{.ChatWebTools}}</li>
          <li><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-link"></use></svg>{{.ChatRichOutput}}</li>
        </ul>
      </div>

      <div class="home-chat-preview" role="img" aria-label="{{.ChatPreviewLabel}}">
        <div class="home-chat-preview-side" aria-hidden="true">
          <span class="home-chat-preview-brand"><img src="/admin/static/tokenflow-logo.png" alt=""><strong>LLM Chat</strong></span>
          <span class="home-chat-preview-new"><svg class="icon"><use href="/admin/static/icons.svg#icon-add"></use></svg>{{.ChatNewConversation}}</span>
          <span class="home-chat-preview-row active"><svg class="icon"><use href="/admin/static/icons.svg#icon-list"></use></svg>{{.ChatConversations}}</span>
        </div>
        <div class="home-chat-preview-main" aria-hidden="true">
          <div class="home-chat-preview-head"><strong>LLM Chat</strong><span><svg class="icon"><use href="/admin/static/icons.svg#icon-route"></use></svg>{{.ChatModelChoice}}</span></div>
          <div class="home-chat-preview-body">
            <p class="home-chat-preview-user">{{.ChatPrompt}}</p>
            <div class="home-chat-preview-answer">
              <img src="/admin/static/tokenflow-logo.png" alt="">
              <div><span>{{.ChatRouteStatus}}</span><i></i><i></i><i></i></div>
            </div>
          </div>
          <div class="home-chat-preview-tools">
            <span><svg class="icon"><use href="/admin/static/icons.svg#icon-search"></use></svg>{{.ChatSearch}}</span>
            <span><svg class="icon"><use href="/admin/static/icons.svg#icon-link"></use></svg>{{.ChatRead}}</span>
            <span><svg class="icon"><use href="/admin/static/icons.svg#icon-list"></use></svg>{{.ChatProcess}}</span>
          </div>
          <div class="home-chat-preview-composer"><span>{{.ChatComposer}}</span><svg class="icon"><use href="/admin/static/icons.svg#icon-arrow-up"></use></svg></div>
        </div>
      </div>
    </section>
  </main>
  <script type="module" src="/admin/static/home/app.js"></script>
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
			Lang:                "zh-CN",
			Title:               "一念通流 TokenFlow · LLM 网关",
			BrandName:           "一念通流",
			FullBrand:           "一念通流 TokenFlow",
			EnglishBrand:        "TokenFlow",
			GatewayLabel:        "TokenFlow",
			AdminLogin:          "管理后台",
			Kicker:              "一念通流，百模可达",
			Headline:            "一个入口，连接你的全部模型",
			Description:         "一念通流 TokenFlow 是一个兼容 OpenAI 与 Anthropic 协议的 LLM 网关。",
			UserLogin:           "登录用户门户",
			UserRegister:        "创建账户",
			RouteTitle:          "请求路由",
			RouteSubtitle:       "兼容两种协议，连接不同上游",
			RequestLabel:        "客户端请求",
			GatewayCore:         "路由核心",
			UpstreamLabel:       "上游服务",
			AuthLabel:           "密钥鉴权",
			RouteLabel:          "模型路由",
			ConvertLabel:        "协议转换",
			StreamLabel:         "流式转发",
			OpenAIUpstream:      "OpenAI 兼容",
			AnthropicUpstream:   "Anthropic 兼容",
			CapabilitiesLabel:   "核心能力",
			EncryptedKeys:       "密钥加密存储",
			CrossProtocol:       "跨协议转换",
			StreamingResponses:  "SSE 流式响应",
			ChatKicker:          "内置对话工作台",
			ChatTitle:           "内置 LLM Chat",
			ChatDescription:     "无需额外客户端，直接使用一念通流已接入的模型完成对话、检索与内容处理。",
			ChatConversations:   "会话管理",
			ChatModelChoice:     "模型选择与推理强度",
			ChatWebTools:        "联网搜索与网页读取",
			ChatRichOutput:      "Markdown 与代码预览",
			ChatPreviewLabel:    "LLM Chat 工作台界面预览",
			ChatNewConversation: "新对话",
			ChatPrompt:          "帮我梳理这份 API 迁移方案",
			ChatRouteStatus:     "通过一念通流路由已接入模型",
			ChatComposer:        "输入消息",
			ChatSearch:          "联网搜索",
			ChatRead:            "网页读取",
			ChatProcess:         "过程",
		}
	}
	return homeData{
		Lang:                "en",
		Title:               "TokenFlow · LLM Gateway",
		BrandName:           "TokenFlow",
		FullBrand:           "TokenFlow",
		GatewayLabel:        "LLM Gateway",
		AdminLogin:          "Admin console",
		Kicker:              "Unified model access",
		Headline:            "One gateway for every model",
		Description:         "Connect through OpenAI or Anthropic APIs while TokenFlow handles authentication, routing, conversion, and streaming.",
		UserLogin:           "Log in to user portal",
		UserRegister:        "Create account",
		RouteTitle:          "Request routing",
		RouteSubtitle:       "Two API protocols, one upstream path",
		RequestLabel:        "Client requests",
		GatewayCore:         "Routing core",
		UpstreamLabel:       "Upstream services",
		AuthLabel:           "Authentication",
		RouteLabel:          "Model routing",
		ConvertLabel:        "Protocol conversion",
		StreamLabel:         "Streaming proxy",
		OpenAIUpstream:      "OpenAI compatible",
		AnthropicUpstream:   "Anthropic compatible",
		CapabilitiesLabel:   "Core capabilities",
		EncryptedKeys:       "Encrypted keys",
		CrossProtocol:       "Cross-protocol conversion",
		StreamingResponses:  "SSE streaming",
		ChatKicker:          "Built-in conversation workspace",
		ChatTitle:           "Built-in LLM Chat",
		ChatDescription:     "Use every connected model for conversations, research, and content work without adding another client.",
		ChatConversations:   "Conversation management",
		ChatModelChoice:     "Model selection and thinking effort",
		ChatWebTools:        "Web search and page reading",
		ChatRichOutput:      "Markdown and code preview",
		ChatPreviewLabel:    "LLM Chat workspace preview",
		ChatNewConversation: "New chat",
		ChatPrompt:          "Help me review this API migration plan",
		ChatRouteStatus:     "Routing through a connected model",
		ChatComposer:        "Ask a question",
		ChatSearch:          "Search",
		ChatRead:            "Read web",
		ChatProcess:         "Process",
	}
}
