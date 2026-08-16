package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"tokenflow/internal/secret"
	"tokenflow/internal/store"
)

func TestServiceSendMessageRunsToolLoopAndRecordsUsage(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer upstream-key" {
			t.Fatalf("unexpected auth header: %q", auth)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "gpt-test" {
			t.Fatalf("unexpected upstream model: %#v", body["model"])
		}
		if body["stream"] != true {
			t.Fatalf("chat service should request streaming upstream responses: %#v", body)
		}
		messages, _ := body["messages"].([]any)
		if len(messages) == 0 {
			t.Fatalf("upstream request did not include messages: %#v", body)
		}
		system, _ := messages[0].(map[string]any)
		systemContent := stringValue(system["content"])
		for _, want := range []string{"TokenFlow's web chat assistant", "Current date: 2026-07-11", "User time zone: Asia/Singapore", "Always answer with sources.", "preferred display name is: Mek"} {
			if !strings.Contains(systemContent, want) {
				t.Fatalf("system prompt missing %q: %s", want, systemContent)
			}
		}
		switch upstreamCalls {
		case 1:
			if body["reasoning_effort"] != "medium" {
				t.Fatalf("thinking effort was not sent: %#v", body)
			}
			if _, ok := body["tools"].([]any); !ok {
				t.Fatalf("tools were not sent: %#v", body)
			}
			if options, _ := body["stream_options"].(map[string]any); options["include_usage"] != true {
				t.Fatalf("OpenAI stream usage was not requested: %#v", body)
			}
			writeSSE(t, w, map[string]any{
				"choices": []any{map[string]any{
					"delta": map[string]any{"reasoning_content": "I should search first."},
				}},
			})
			writeSSE(t, w, map[string]any{
				"choices": []any{map[string]any{
					"delta": map[string]any{
						"tool_calls": []any{map[string]any{
							"index": 0,
							"id":    "call_1",
							"type":  "function",
							"function": map[string]any{
								"name":      "web_search",
								"arguments": `{"query":"TokenFlow chat","count":2}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
			writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 2, "prompt_tokens_details": map[string]any{"cached_tokens": 3}}})
			writeSSEDone(t, w)
		case 2:
			var sawTool bool
			for _, item := range messages {
				msg, _ := item.(map[string]any)
				if msg["role"] == "tool" && strings.Contains(stringValue(msg["content"]), "Example result") {
					sawTool = true
				}
			}
			if !sawTool {
				t.Fatalf("second request did not include tool result: %#v", messages)
			}
			writeSSE(t, w, map[string]any{
				"choices": []any{map[string]any{
					"delta": map[string]any{"content": "Found a source: "},
				}},
			})
			writeSSE(t, w, map[string]any{
				"choices": []any{map[string]any{
					"delta":         map[string]any{"content": "https://example.com/result"},
					"finish_reason": "stop",
				}},
			})
			writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 20, "completion_tokens": 5}})
			writeSSEDone(t, w)
		default:
			t.Fatalf("unexpected upstream call %d", upstreamCalls)
		}
	}))
	defer upstream.Close()

	var infoFlowCalls int
	infoFlow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		infoFlowCalls++
		if r.URL.Path != "/v1/web_search" {
			t.Fatalf("unexpected InfoFlow path: %s", r.URL.Path)
		}
		writeJSON(t, w, map[string]any{"results": []any{map[string]any{"title": "Example result", "url": "https://example.com/result"}}})
	}))
	defer infoFlow.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "test", "gpt-test", "medium", "Always answer with sources.", "Mek", "😎", "🤖", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, infoFlow.URL)
	service.now = func() time.Time { return time.Date(2026, 7, 10, 16, 30, 0, 0, time.UTC) }
	var events []string
	msg, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{
		Content:      "Find TokenFlow chat info",
		EnableSearch: true,
		EnableRead:   true,
		TimeZone:     "Asia/Singapore",
	}, func(event string, payload any) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 2 || infoFlowCalls != 1 {
		t.Fatalf("unexpected upstream/tool calls: upstream=%d infoflow=%d", upstreamCalls, infoFlowCalls)
	}
	if !strings.Contains(msg.Content, "https://example.com/result") {
		t.Fatalf("unexpected assistant message: %#v", msg)
	}
	for _, want := range []string{"tool_start", "tool_result", "thinking", "delta", "assistant_message", "done"} {
		if !contains(events, want) {
			t.Fatalf("missing event %q in %#v", want, events)
		}
	}
	if countValues(events, "delta") < 2 {
		t.Fatalf("assistant content was not streamed in multiple chunks: %#v", events)
	}
	messages, err := st.ChatMessages(ctx, owner, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Role != store.ChatRoleUser || messages[1].Role != store.ChatRoleAssistant {
		t.Fatalf("chat messages were not persisted: %#v", messages)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(messages[1].Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if events, _ := metadata["events"].([]any); len(events) < 3 {
		t.Fatalf("tool timeline was not stored in metadata: %#v", metadata)
	}
	admin, err = st.AdminUser(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if admin.RequestCount != 2 || admin.InputTokens != 30 || admin.OutputTokens != 7 || admin.CacheReadTokens != 3 {
		t.Fatalf("admin usage was not aggregated across loop: %#v", admin)
	}
	logs, err := st.Logs(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].AdminUsername != admin.Username || logs[1].Protocol != "chat" {
		t.Fatalf("chat request logs were not recorded: %#v", logs)
	}
}

func TestServiceSendMessageSummarizesFirstConversationTitle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch upstreamCalls {
		case 1:
			if _, ok := body["tools"]; ok {
				t.Fatalf("tools should not be sent when request toggles are off: %#v", body)
			}
			writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "Here is the answer."}}}})
			writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 11, "completion_tokens": 4}})
			writeSSEDone(t, w)
		case 2:
			if body["max_tokens"] != float64(64) {
				t.Fatalf("title request should be low token: %#v", body)
			}
			if _, ok := body["tools"]; ok {
				t.Fatalf("title request should not include tools: %#v", body)
			}
			messages, _ := body["messages"].([]any)
			if len(messages) != 2 || !strings.Contains(stringValue(messages[0].(map[string]any)["content"]), "concise chat title") {
				t.Fatalf("unexpected title summary messages: %#v", messages)
			}
			writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "TokenFlow 标题"}}}})
			writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 3}})
			writeSSEDone(t, w)
		default:
			t.Fatalf("unexpected upstream call %d", upstreamCalls)
		}
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "新对话", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if !conv.TitleAutoGenerated {
		t.Fatalf("localized default title should allow auto summary: %#v", conv)
	}

	service := NewService(st, box, "")
	var conversationEvents []store.ChatConversation
	var autoTitle bool
	requestCtx, cancelRequest := context.WithCancel(ctx)
	defer cancelRequest()
	if _, err := service.SendMessage(requestCtx, owner, conv.ID, SendRequest{Content: "帮我解释 TokenFlow", EnableSearch: false, EnableRead: false}, func(event string, payload any) error {
		if event == "conversation" {
			if conv, ok := payload.(store.ChatConversation); ok {
				conversationEvents = append(conversationEvents, conv)
			}
		}
		if event == "done" {
			if body, ok := payload.(map[string]any); ok {
				autoTitle, _ = body["auto_title"].(bool)
			}
			cancelRequest()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 1 || !autoTitle {
		t.Fatalf("reply SSE should finish before title generation: calls=%d auto_title=%v", upstreamCalls, autoTitle)
	}
	if _, err := service.GenerateConversationTitle(ctx, owner, conv.ID, false); err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 2 {
		t.Fatalf("expected separate title request, got %d calls", upstreamCalls)
	}
	updated, err := st.ChatConversation(ctx, owner, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "TokenFlow 标题" || !updated.TitleAutoGenerated {
		t.Fatalf("conversation title was not summarized: %#v", updated)
	}
	if len(conversationEvents) == 0 {
		t.Fatalf("reply operation should emit conversation state: %#v", conversationEvents)
	}
	admin, err = st.AdminUser(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if admin.RequestCount != 2 || admin.InputTokens != 19 || admin.OutputTokens != 7 {
		t.Fatalf("title request usage was not recorded: %#v", admin)
	}
}

func TestServiceSendMessageOmitsToolsWhenConversationLimitIsZero(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, hasTools := body["tools"]
		if upstreamCalls == 1 && hasTools {
			t.Fatalf("tools should be omitted when max_tool_calls is 0: %#v", body)
		}
		if upstreamCalls == 2 && !hasTools {
			t.Fatalf("tools should be available for a conversation with the default limit: %#v", body)
		}
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "No tools used."}}}})
		writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 3}})
		writeSSEDone(t, w)
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "manual", "gpt-test", "medium", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(st, box, "")
	if _, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{Content: "Search anyway", EnableSearch: true, EnableRead: true}, nil); err != nil {
		t.Fatal(err)
	}
	defaultConv, err := st.CreateChatConversation(ctx, owner, "default limit", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SendMessage(ctx, owner, defaultConv.ID, SendRequest{Content: "Search with tools", EnableSearch: true, EnableRead: true}, nil); err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 2 {
		t.Fatalf("each manual-title conversation should make one upstream request, got %d", upstreamCalls)
	}
}

func TestServiceSendMessageTitleSummaryFailureDoesNotBlockReply(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		switch upstreamCalls {
		case 1:
			writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "Answer before title failure."}}}})
			writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 4}})
			writeSSEDone(t, w)
		case 2:
			http.Error(w, "title unavailable", http.StatusBadGateway)
		default:
			t.Fatalf("unexpected upstream call %d", upstreamCalls)
		}
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}

	var sawAssistant bool
	var sawDone bool
	var autoTitle bool
	service := NewService(st, box, "")
	msg, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{Content: "Trigger title failure", EnableSearch: false, EnableRead: false}, func(event string, payload any) error {
		switch event {
		case "assistant_message":
			sawAssistant = true
		case "done":
			sawDone = true
			if body, ok := payload.(map[string]any); ok {
				autoTitle, _ = body["auto_title"].(bool)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Answer before title failure." || !sawAssistant || !sawDone || !autoTitle {
		t.Fatalf("title scheduling should not block assistant reply: msg=%#v assistant=%v done=%v auto_title=%v", msg, sawAssistant, sawDone, autoTitle)
	}
	if _, err := service.GenerateConversationTitle(ctx, owner, conv.ID, false); err == nil {
		t.Fatal("separate title request should report the upstream failure")
	}
	messages, err := st.ChatMessages(ctx, owner, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].Role != store.ChatRoleAssistant {
		t.Fatalf("assistant message was not persisted after title failure: %#v", messages)
	}
}

func TestServiceGenerateConversationTitleForceOverwritesManualTitle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["tools"]; ok {
			t.Fatalf("title request should not include tools: %#v", body)
		}
		messages, _ := body["messages"].([]any)
		if len(messages) != 2 || !strings.Contains(stringValue(messages[1].(map[string]any)["content"]), "Manual title request") || !strings.Contains(stringValue(messages[1].(map[string]any)["content"]), "Assistant answer") {
			t.Fatalf("title request should include conversation history: %#v", messages)
		}
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "手动生成标题"}}}})
		writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 6, "completion_tokens": 2}})
		writeSSEDone(t, w)
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "Manual title", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateChatMessage(ctx, owner, conv.ID, store.ChatRoleUser, "Manual title request", "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateChatMessage(ctx, owner, conv.ID, store.ChatRoleAssistant, "Assistant answer", "{}"); err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, "")
	if unchanged, err := service.GenerateConversationTitle(ctx, owner, conv.ID, false); err != nil || unchanged.Title != "Manual title" || unchanged.TitleAutoGenerated {
		t.Fatalf("non-forced generation should keep manual title, conv=%#v err=%v", unchanged, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("non-forced manual title should not call upstream, got %d", upstreamCalls)
	}
	updated, err := service.GenerateConversationTitle(ctx, owner, conv.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 1 || updated.Title != "手动生成标题" || !updated.TitleAutoGenerated {
		t.Fatalf("forced title generation failed: calls=%d conv=%#v", upstreamCalls, updated)
	}
}

func TestServiceGenerateConversationTitleRetriesEmptyAndFallsBack(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch upstreamCalls {
		case 1:
			if body["stream"] != true {
				t.Fatalf("first title request should stream: %#v", body)
			}
			writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": ""}}}})
			writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 0}})
			writeSSEDone(t, w)
		case 2:
			if body["stream"] != false {
				t.Fatalf("second title request should retry without streaming: %#v", body)
			}
			writeJSON(t, w, map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": ""}}},
				"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 0},
			})
		default:
			t.Fatalf("unexpected upstream call %d", upstreamCalls)
		}
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateChatMessage(ctx, owner, conv.ID, store.ChatRoleUser, "请总结 TokenFlow 的管理员用量统计设计", "{}"); err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, "")
	updated, err := service.GenerateConversationTitle(ctx, owner, conv.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 2 {
		t.Fatalf("expected streamed request plus non-stream retry, got %d", upstreamCalls)
	}
	if updated.Title != "请总结 TokenFlow 的管理员用量统计设计" {
		t.Fatalf("empty generated title should fall back to first user message: %#v", updated)
	}
}

func TestServiceConversationBusyRejectsSendAndTitle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("busy conversation should not call upstream")
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateChatMessage(ctx, owner, conv.ID, store.ChatRoleUser, "Need a title", "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.StartChatConversationOperation(ctx, owner, conv.ID, store.ChatConversationOperationResponding, "", 30*time.Minute); err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, "")
	if _, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{Content: "second message"}, nil); !errors.Is(err, store.ErrChatConversationBusy) {
		t.Fatalf("busy send should return ErrChatConversationBusy, got %v", err)
	}
	if _, err := service.GenerateConversationTitle(ctx, owner, conv.ID, true); !errors.Is(err, store.ErrChatConversationBusy) {
		t.Fatalf("busy title should return ErrChatConversationBusy, got %v", err)
	}
}

func TestServiceAllowsDifferentConversationWhileOneIsBusy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "second conversation reply"}}}})
		writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 2, "completion_tokens": 3}})
		writeSSEDone(t, w)
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	busyConv, err := st.CreateChatConversation(ctx, owner, "busy", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	freeConv, err := st.CreateChatConversation(ctx, owner, "free", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.StartChatConversationOperation(ctx, owner, busyConv.ID, store.ChatConversationOperationResponding, "", 30*time.Minute); err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, "")
	msg, err := service.SendMessage(ctx, owner, freeConv.ID, SendRequest{Content: "hello from free conversation"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "second conversation reply" || upstreamCalls != 1 {
		t.Fatalf("different conversation should complete independently: calls=%d msg=%#v", upstreamCalls, msg)
	}
}

func TestServiceGenerateConversationTitleRequiresMessages(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		t.Fatalf("empty conversation should not call upstream")
	}))
	defer upstream.Close()

	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "mock",
		Protocol:     "openai",
		BaseAPI:      upstream.URL + "/v1",
		APIKeyCipher: cipher,
		DefaultModel: "gpt-test",
		Models:       []string{"gpt-test"},
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, "")
	if _, err := service.GenerateConversationTitle(ctx, owner, conv.ID, true); err != ErrNoTitleMessages {
		t.Fatalf("expected ErrNoTitleMessages, got %v", err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("empty title generation should not call upstream, got %d", upstreamCalls)
	}
}

func TestChatMessageUnicodeLimit(t *testing.T) {
	valid := strings.Repeat("界", MaxUserMessageRunes)
	if got, err := validateMessageContent(valid); err != nil || len([]rune(got)) != MaxUserMessageRunes {
		t.Fatalf("128K Unicode message should be accepted: runes=%d err=%v", len([]rune(got)), err)
	}
	if _, err := validateMessageContent(valid + "界"); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("128K+1 Unicode message should fail with ErrMessageTooLarge: %v", err)
	}
	emoji := strings.Repeat("😀", MaxUserMessageRunes)
	if got, err := validateMessageContent(emoji); err != nil || len([]rune(got)) != MaxUserMessageRunes {
		t.Fatalf("128K emoji message should count code points, not UTF-16 units: runes=%d err=%v", len([]rune(got)), err)
	}
	if _, err := validateMessageContent("  \n\t "); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("blank message should fail with ErrEmptyMessage: %v", err)
	}
}

func TestChatMessageHTTPRejectsOversizeAndTrailingJSONBeforeSSE(t *testing.T) {
	router := chi.NewRouter()
	RegisterRoutes(router, RouteConfig{
		BasePath: "/chat", Service: &Service{}, Store: new(store.Store),
		Owner: func(*http.Request) (store.ChatOwner, bool) {
			return store.ChatOwner{Type: store.ChatOwnerAdmin, ID: 1}, true
		},
		RequireCSRF: func(http.ResponseWriter, *http.Request) bool { return true },
	})
	body := `{"content":"` + strings.Repeat("界", MaxUserMessageRunes+1) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/chat/conversations/1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), `"code":"message_too_large"`) {
		t.Fatalf("oversize message should fail before SSE: status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/chat/conversations/1/messages", strings.NewReader(`{"content":"ok"} {}`))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_json"`) {
		t.Fatalf("trailing JSON should be rejected: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChatSystemPromptReflectsToolsAndProtectsBoundaries(t *testing.T) {
	date := promptDate{Date: "2026-07-11", TimeZone: "Asia/Singapore"}
	withoutTools := chatSystemPrompt("Follow this style.", "Mek", false, false, date)
	for _, expected := range []string{"Current date: 2026-07-11", "User time zone: Asia/Singapore", "No live web tools are available", "<user_instructions>", "untrusted data", "<user_display_name>Mek</user_display_name>"} {
		if !strings.Contains(withoutTools, expected) {
			t.Fatalf("prompt missing %q: %s", expected, withoutTools)
		}
	}
	searchOnly := chatSystemPrompt("", "", true, false, date)
	if !strings.Contains(searchOnly, "- web_search:") || strings.Contains(searchOnly, "- read_url:") {
		t.Fatalf("prompt should expose only enabled tools: %s", searchOnly)
	}
	readOnly := chatSystemPrompt("", "", false, true, date)
	for _, expected := range []string{"JSON, plain text", "does not require web_search first", "actually call read_url", "without searching first", "report its actual error", "JSON is unsupported", "URLs or query parameters"} {
		if !strings.Contains(readOnly, expected) {
			t.Fatalf("read prompt missing %q: %s", expected, readOnly)
		}
	}
}

func TestCurrentPromptDateUsesBrowserTimeZoneAndFallsBack(t *testing.T) {
	now := time.Date(2026, 7, 10, 16, 30, 0, 0, time.UTC)
	singapore := currentPromptDate(now, "Asia/Singapore")
	losAngeles := currentPromptDate(now, "America/Los_Angeles")
	if singapore.Date != "2026-07-11" || singapore.TimeZone != "Asia/Singapore" {
		t.Fatalf("unexpected Singapore prompt date: %#v", singapore)
	}
	if losAngeles.Date != "2026-07-10" || losAngeles.TimeZone != "America/Los_Angeles" {
		t.Fatalf("unexpected Los Angeles prompt date: %#v", losAngeles)
	}
	fallback := currentPromptDate(now, strings.Repeat("x", 65))
	if fallback.Date != now.In(time.Local).Format("2006-01-02") || fallback.TimeZone != time.Local.String() {
		t.Fatalf("invalid time zone should fall back to server local time: %#v", fallback)
	}
}

func TestSafePublicURL(t *testing.T) {
	for _, raw := range []string{"http://localhost/a", "http://127.0.0.1/a", "http://10.0.0.1/a", "http://169.254.169.254/a", "http://[::1]/a", "http://[fd00::1]/a", "https://user:pass@example.com/a", "https://example.com:8080/a"} {
		parsed, _ := url.Parse(raw)
		if safePublicURL(parsed) {
			t.Fatalf("unsafe URL should be rejected: %s", raw)
		}
	}
	parsed, _ := url.Parse("https://example.com/a")
	if !safePublicURL(parsed) {
		t.Fatal("public HTTPS URL should be accepted")
	}
}

func TestReadURLAllowsArbitraryPublicJSONAndPreservesURL(t *testing.T) {
	const target = "https://example.com/releases/app.json?channel=stable&lang=zh-CN"
	infoFlow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/read_url" {
			t.Fatalf("unexpected InfoFlow request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["url"] != target {
			t.Fatalf("read_url changed path or query: %#v", payload)
		}
		if payload["block_private_networks"] != true || payload["max_redirects"] != float64(5) {
			t.Fatalf("read_url must retain private-network and redirect protection: %#v", payload)
		}
		writeJSON(t, w, map[string]any{
			"url":                 target,
			"final_url":           target,
			"source_content_type": "application/json",
			"markdown":            "```json\n{\"appVersion\":\"1.2.3\",\"installUrl\":\"https://cdn.example.com/app.apk\"}\n```",
		})
	}))
	defer infoFlow.Close()

	service := &Service{infoFlowBaseURL: infoFlow.URL, infoFlowClient: infoFlow.Client()}
	result, ok := service.executeTool(context.Background(), SendRequest{Content: "No URL appears here", EnableRead: true}, toolCall{
		Name:      "read_url",
		Arguments: `{"url":"` + target + `","render":false}`,
	})
	if !ok || !strings.Contains(result, `"source_content_type":"application/json"`) || !strings.Contains(result, `appVersion`) || !strings.Contains(result, `installUrl`) {
		t.Fatalf("public JSON should be returned as tool content: ok=%v result=%s", ok, result)
	}
}

func TestServicePassesArbitraryPublicJSONIntoNextModelCall(t *testing.T) {
	const target = "https://example.com/releases/app.json?channel=stable"
	var upstreamCalls int
	service, _, owner, conv := newGenerationTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch upstreamCalls {
		case 1:
			writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": "call_json", "type": "function", "function": map[string]any{"name": "read_url", "arguments": `{"url":"` + target + `","render":false}`}}}}, "finish_reason": "tool_calls"}}})
			writeSSEDone(t, w)
		case 2:
			messages, _ := body["messages"].([]any)
			var toolContent string
			for _, item := range messages {
				message, _ := item.(map[string]any)
				if message["role"] == "tool" {
					toolContent = stringValue(message["content"])
				}
			}
			for _, expected := range []string{"application/json", "appVersion", "1.2.3", "installUrl", "app.apk"} {
				if !strings.Contains(toolContent, expected) {
					t.Fatalf("JSON tool result missing %q in next model request: %s", expected, toolContent)
				}
			}
			writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "Version 1.2.3"}, "finish_reason": "stop"}}})
			writeSSEDone(t, w)
		default:
			t.Fatalf("unexpected upstream call %d", upstreamCalls)
		}
	}))

	infoFlow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if r.URL.Path != "/v1/read_url" || payload["url"] != target {
			t.Fatalf("unexpected JSON read request: path=%s payload=%#v", r.URL.Path, payload)
		}
		writeJSON(t, w, map[string]any{"url": target, "final_url": target, "source_content_type": "application/json", "markdown": "```json\n{\"appVersion\":\"1.2.3\",\"installUrl\":\"https://cdn.example.com/app.apk\"}\n```"})
	}))
	defer infoFlow.Close()
	service.infoFlowBaseURL = infoFlow.URL
	service.infoFlowClient = infoFlow.Client()

	message, err := service.SendMessage(context.Background(), owner, conv.ID, SendRequest{Content: "Read the release metadata", EnableRead: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 2 || message.Content != "Version 1.2.3" {
		t.Fatalf("unexpected JSON tool loop result: calls=%d message=%#v", upstreamCalls, message)
	}
}

func TestBuildOpenAIMessagesRehydratesToolContext(t *testing.T) {
	metadata := `{"completion_status":"completed","events":[{"type":"tool_start","id":"call_1","name":"web_search","arguments":"{\"query\":\"tokenflow\"}"},{"type":"tool_result","id":"call_1","name":"web_search","ok":true,"result":"{\"url\":\"https://example.com\"}"}]}`
	history := []store.ChatMessage{
		{ID: 1, Role: store.ChatRoleUser, Content: "search"},
		{ID: 2, Role: store.ChatRoleAssistant, Content: "result", Metadata: metadata},
	}
	messages, omitted, err := buildOpenAIMessages(history, store.ChatConversation{}, true, false, promptDate{Date: "2026-07-11", TimeZone: "UTC"}, 100000)
	if err != nil || len(omitted) != 0 {
		t.Fatalf("unexpected build result: omitted=%#v err=%v", omitted, err)
	}
	roles := make([]string, 0, len(messages))
	for _, raw := range messages {
		if msg, ok := raw.(map[string]any); ok {
			roles = append(roles, stringArg(msg, "role", ""))
		}
	}
	joined := strings.Join(roles, ",")
	if joined != "system,user,assistant,tool,assistant" {
		t.Fatalf("tool context was not reconstructed: %s %#v", joined, messages)
	}
}

func TestBuildOpenAIMessagesBudgetsOldTurnsAndPreservesLatest(t *testing.T) {
	history := []store.ChatMessage{
		{ID: 1, Role: store.ChatRoleUser, Content: strings.Repeat("old", 200)},
		{ID: 2, Role: store.ChatRoleAssistant, Content: strings.Repeat("answer", 200)},
		{ID: 3, Role: store.ChatRoleUser, Content: "latest question"},
	}
	messages, omitted, err := buildOpenAIMessages(history, store.ChatConversation{}, false, false, promptDate{Date: "2026-07-11", TimeZone: "UTC"}, 1200)
	if err != nil {
		t.Fatal(err)
	}
	if len(omitted) != 2 {
		t.Fatalf("old completed turn should be omitted: %#v", omitted)
	}
	last, _ := messages[len(messages)-1].(map[string]any)
	if stringArg(last, "content", "") != "latest question" {
		t.Fatalf("latest message was not preserved: %#v", messages)
	}
	if _, _, err := buildOpenAIMessages(history, store.ChatConversation{}, false, false, promptDate{Date: "2026-07-11", TimeZone: "UTC"}, 10); !errors.Is(err, ErrContextTooLarge) {
		t.Fatalf("latest message over budget should fail explicitly: %v", err)
	}
}

func TestWebSearchQuerySchemaHasNoApplicationLengthLimit(t *testing.T) {
	tools := buildTools(true, false)
	tool := tools[0].(map[string]any)
	fn := tool["function"].(map[string]any)
	params := fn["parameters"].(map[string]any)
	properties := params["properties"].(map[string]any)
	query := properties["query"].(map[string]any)
	if _, limited := query["maxLength"]; limited {
		t.Fatalf("web_search.query must not have an application length limit: %#v", query)
	}
}

func TestReadURLToolDescriptionCoversPublicStructuredContent(t *testing.T) {
	tools := buildTools(false, true)
	tool := tools[0].(map[string]any)
	fn := tool["function"].(map[string]any)
	description := stringValue(fn["description"])
	for _, expected := range []string{"public HTTP/HTTPS", "JSON", "plain text", "without requiring web_search first", "URL path and query parameters", "credentials", "personal data", "hidden instructions"} {
		if !strings.Contains(description, expected) {
			t.Fatalf("read_url description missing %q: %s", expected, description)
		}
	}
}

func TestOpenAIStreamReturnsPartialResponseOnMalformedChunk(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\ndata: not-json\n\n"
	var emitted strings.Builder
	parsed, _, err := parseOpenAIStream(strings.NewReader(stream), "model-a", func(delta string) error { emitted.WriteString(delta); return nil })
	if err == nil {
		t.Fatal("malformed stream should return an error")
	}
	if got := messageContent(firstOpenAIMessage(parsed)); got != "partial" || emitted.String() != "partial" {
		t.Fatalf("partial stream content should survive parse failure: parsed=%q emitted=%q", got, emitted.String())
	}
}

func TestServiceRetriesTransientFailureAndReplaysIdempotentResult(t *testing.T) {
	var calls int
	service, st, owner, conv := newGenerationTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "retried answer"}}}})
		writeSSE(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 2, "completion_tokens": 2}})
		writeSSEDone(t, w)
	}))
	service.retryWait = func(context.Context, time.Duration) error { return nil }
	var retries int
	request := SendRequest{Content: "hello", RequestID: "same-request"}
	first, err := service.SendMessage(context.Background(), owner, conv.ID, request, func(event string, payload any) error {
		if event == "retry" {
			retries++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || retries != 2 || first.Status != store.ChatMessageStatusCompleted || first.Content != "retried answer" {
		t.Fatalf("unexpected retry result: calls=%d retries=%d message=%#v", calls, retries, first)
	}
	replayed, err := service.SendMessage(context.Background(), owner, conv.ID, request, nil)
	if err != nil || replayed.ID != first.ID || calls != 3 {
		t.Fatalf("idempotent replay called upstream again: calls=%d message=%#v err=%v", calls, replayed, err)
	}
	messages, err := st.ChatMessages(context.Background(), owner, conv.ID)
	if err != nil || len(messages) != 2 {
		t.Fatalf("idempotent turn should contain two messages: %#v err=%v", messages, err)
	}
}

func TestServiceDoesNotRetryAfterVisibleDelta(t *testing.T) {
	var calls int
	service, st, owner, conv := newGenerationTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if _, err := fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\ndata: not-json\n\n"); err != nil {
			t.Error(err)
		}
	}))
	service.retryWait = func(context.Context, time.Duration) error { return nil }
	if _, err := service.SendMessage(context.Background(), owner, conv.ID, SendRequest{Content: "partial failure", RequestID: "partial-request"}, nil); err == nil {
		t.Fatal("malformed stream should fail")
	}
	if calls != 1 {
		t.Fatalf("visible partial response must not be retried, calls=%d", calls)
	}
	messages, err := st.ChatMessages(context.Background(), owner, conv.ID)
	if err != nil || len(messages) != 2 || messages[1].Content != "partial" || messages[1].Status != store.ChatMessageStatusFailed {
		t.Fatalf("partial failure was not persisted: %#v err=%v", messages, err)
	}
}

func TestServiceStopPersistsPartialAssistant(t *testing.T) {
	service, st, owner, conv := newGenerationTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "partial"}}}})
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	deltaSeen := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := service.SendMessage(context.Background(), owner, conv.ID, SendRequest{Content: "stop me", RequestID: "stop-request"}, func(event string, payload any) error {
			if event == "delta" {
				select {
				case <-deltaSeen:
				default:
					close(deltaSeen)
				}
			}
			return nil
		})
		done <- err
	}()
	select {
	case <-deltaSeen:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for streamed content")
	}
	stopped, err := service.StopConversation(context.Background(), owner, conv.ID)
	if err != nil || !stopped {
		t.Fatalf("stop failed: stopped=%v err=%v", stopped, err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled generation should return an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("generation did not stop")
	}
	messages, err := st.ChatMessages(context.Background(), owner, conv.ID)
	if err != nil || len(messages) != 2 || messages[1].Content != "partial" || messages[1].Status != store.ChatMessageStatusStopped {
		t.Fatalf("partial stopped response was not persisted: %#v err=%v", messages, err)
	}
	updated, err := st.ChatConversation(context.Background(), owner, conv.ID)
	if err != nil || updated.ActiveOperation != "" || updated.Status != store.ChatConversationStatusStopped {
		t.Fatalf("conversation lock was not released: %#v err=%v", updated, err)
	}
}

func TestServiceRegeneratesOnlyLatestAssistant(t *testing.T) {
	var calls int
	service, st, owner, conv := newGenerationTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := "first answer"
		if calls == 2 {
			content = "replacement answer"
		}
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": content}}}})
		writeSSEDone(t, w)
	}))
	first, err := service.SendMessage(context.Background(), owner, conv.ID, SendRequest{Content: "original prompt", RequestID: "initial"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	replaced, err := service.RegenerateMessage(context.Background(), owner, conv.ID, first.ID, SendRequest{RequestID: "replacement"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := st.ChatMessages(context.Background(), owner, conv.ID)
	if err != nil || len(messages) != 2 || messages[0].Content != "original prompt" || messages[1].ID != first.ID || replaced.Content != "replacement answer" {
		t.Fatalf("unexpected regenerated turn: %#v replacement=%#v err=%v", messages, replaced, err)
	}
	if _, err := st.CreateChatMessage(context.Background(), owner, conv.ID, store.ChatRoleUser, "later", "{}"); err != nil {
		t.Fatal(err)
	}
	if err := service.PreflightRegenerate(context.Background(), owner, conv.ID, first.ID); !errors.Is(err, ErrMessageNotLatest) {
		t.Fatalf("non-latest regeneration should fail, got %v", err)
	}
}

func TestChatLifecycleRoutesEnforceCSRFAndStreamRegeneration(t *testing.T) {
	service, st, owner, conv := newGenerationTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(t, w, map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "answer"}}}})
		writeSSEDone(t, w)
	}))
	message, err := service.SendMessage(context.Background(), owner, conv.ID, SendRequest{Content: "prompt", RequestID: "route-initial"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterRoutes(router, RouteConfig{
		BasePath: "/chat", Service: service, Store: st,
		Owner: func(*http.Request) (store.ChatOwner, bool) { return owner, true },
		RequireCSRF: func(w http.ResponseWriter, r *http.Request) bool {
			if r.Header.Get("X-CSRF-Token") == "valid" {
				return true
			}
			w.WriteHeader(http.StatusForbidden)
			return false
		},
	})
	path := fmt.Sprintf("/chat/conversations/%d/messages/%d/regenerate", conv.ID, message.ID)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"request_id":"route-regenerate"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("regenerate without CSRF should fail: status=%d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"request_id":"route-regenerate"}`))
	req.Header.Set("X-CSRF-Token", "valid")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("regenerate route did not stream completion: status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/chat/conversations/%d/stop", conv.ID), strings.NewReader(`{}`))
	req.Header.Set("X-CSRF-Token", "valid")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"stopped":false`) {
		t.Fatalf("idle stop should be idempotent: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func newGenerationTestService(t *testing.T, upstreamHandler http.Handler) (*Service, *store.Store, store.ChatOwner, store.ChatConversation) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	box, err := secret.Load(filepath.Join(dir, "app.secret"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)
	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProvider(ctx, store.ProviderInput{Name: "mock", Protocol: "openai", BaseAPI: upstream.URL + "/v1", APIKeyCipher: cipher, DefaultModel: "gpt-test", Models: []string{"gpt-test"}, Enabled: true, IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, "generation-admin", "hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := st.AdminByUsername(ctx, "generation-admin")
	if err != nil {
		t.Fatal(err)
	}
	owner := store.ChatOwner{Type: store.ChatOwnerAdmin, ID: admin.ID, Name: admin.Username}
	conv, err := st.CreateChatConversation(ctx, owner, "test", "gpt-test", "medium", "", "", "", "", store.DefaultChatMaxToolCalls)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(st, box, ""), st, owner, conv
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatal(err)
	}
}

func writeSSE(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
		t.Fatal(err)
	}
}

func writeSSEDone(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		t.Fatal(err)
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return value.(string)
}

func countValues(values []string, want string) int {
	var count int
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
