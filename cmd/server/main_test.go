package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeShowsPortalInsteadOfRedirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")
	rec := httptest.NewRecorder()

	home(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("home should render a page, got status %d", rec.Code)
	}
	if rec.Header().Get("Location") != "" {
		t.Fatalf("home should not redirect, got Location %q", rec.Header().Get("Location"))
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`<html lang="zh-CN">`,
		`<body class="home-page">`,
		`href="/admin/static/css/home.css"`,
		`src="/admin/static/home/app.js"`,
		`src="/admin/static/tokenflow-logo.svg"`,
		`href="/admin/static/icons.svg#icon-route"`,
		`class="route-flow-highlight route-request-highlight route-flow-animated"`,
		`class="route-connector route-connector-inbound"`,
		`class="route-progress route-flow-animated"`,
		`class="route-connector route-connector-outbound"`,
		`class="route-flow-highlight route-upstream-highlight route-flow-animated"`,
		`class="home-chat-showcase" data-home-motion`,
		`class="home-chat-preview" role="img"`,
		`class="home-chat-preview-side"`,
		`class="home-chat-preview-tools"`,
		`<h1 id="home-title">一念通流</h1>`,
		`<p class="home-english-brand">TokenFlow</p>`,
		"一念通流，百模可达",
		"一个入口，连接你的全部模型",
		"请求路由",
		"客户端请求",
		"内置 LLM Chat",
		"无需额外客户端，直接使用一念通流已接入的模型完成对话、检索与内容处理。",
		"会话管理",
		"模型选择与推理强度",
		"联网搜索与网页读取",
		"Markdown 与代码预览",
		"登录用户门户",
		"管理后台",
		`/v1/chat/completions`,
		`/v1/messages`,
		`href="/account/login"`,
		`href="/account/register"`,
		`href="/admin"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in home page:\n%s", expected, body)
		}
	}
	if strings.Contains(body, `href="/account/chat"`) {
		t.Fatalf("home Chat introduction should not add a Chat link:\n%s", body)
	}
}

func TestHomeDefaultsToEnglish(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	home(rec, req)

	body := rec.Body.String()
	for _, expected := range []string{
		`<html lang="en">`,
		`<h1 id="home-title">TokenFlow</h1>`,
		"One gateway for every model",
		"Request routing",
		"Client requests",
		"Built-in LLM Chat",
		"Use every connected model for conversations, research, and content work without adding another client.",
		"Conversation management",
		"Model selection and thinking effort",
		"Web search and page reading",
		"Markdown and code preview",
		"Log in to user portal",
		"Admin console",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in home page:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "一念通流") || strings.Contains(body, "home-english-brand") {
		t.Fatalf("English home page should not force the Chinese brand:\n%s", body)
	}
}
