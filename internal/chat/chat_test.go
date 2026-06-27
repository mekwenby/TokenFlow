package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		for _, want := range []string{"TokenFlow's web chat assistant", "Always answer with sources.", "preferred display name is: Mek"} {
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
	conv, err := st.CreateChatConversation(ctx, owner, "test", "gpt-test", "medium", "Always answer with sources.", "Mek", "😎", "🤖")
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(st, box, infoFlow.URL)
	var events []string
	msg, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{
		Content:        "Find TokenFlow chat info",
		Model:          "gpt-test",
		ThinkingEffort: "medium",
		EnableSearch:   true,
		EnableRead:     true,
		ShowProcess:    true,
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
	conv, err := st.CreateChatConversation(ctx, owner, "新对话", "gpt-test", "medium", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !conv.TitleAutoGenerated {
		t.Fatalf("localized default title should allow auto summary: %#v", conv)
	}

	service := NewService(st, box, "")
	var conversationEvents []store.ChatConversation
	requestCtx, cancelRequest := context.WithCancel(ctx)
	defer cancelRequest()
	if _, err := service.SendMessage(requestCtx, owner, conv.ID, SendRequest{Content: "帮我解释 TokenFlow", EnableSearch: false, EnableRead: false}, func(event string, payload any) error {
		if event == "conversation" {
			if conv, ok := payload.(store.ChatConversation); ok {
				conversationEvents = append(conversationEvents, conv)
			}
		}
		if event == "done" {
			cancelRequest()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 2 {
		t.Fatalf("expected answer and title requests, got %d", upstreamCalls)
	}
	updated, err := st.ChatConversation(ctx, owner, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "TokenFlow 标题" || !updated.TitleAutoGenerated {
		t.Fatalf("conversation title was not summarized: %#v", updated)
	}
	if len(conversationEvents) < 2 || conversationEvents[len(conversationEvents)-1].Title != "TokenFlow 标题" {
		t.Fatalf("title update was not emitted as a conversation event: %#v", conversationEvents)
	}
	admin, err = st.AdminUser(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if admin.RequestCount != 2 || admin.InputTokens != 19 || admin.OutputTokens != 7 {
		t.Fatalf("title request usage was not recorded: %#v", admin)
	}
}

func TestServiceSendMessageOmitsToolsWhenGlobalLimitIsZero(t *testing.T) {
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
	if _, err := st.UpdateChatMaxToolCalls(ctx, 0); err != nil {
		t.Fatal(err)
	}

	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["tools"]; ok {
			t.Fatalf("tools should be omitted when max_tool_calls is 0: %#v", body)
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
	conv, err := st.CreateChatConversation(ctx, owner, "manual", "gpt-test", "medium", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(st, box, "")
	if _, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{Content: "Search anyway", EnableSearch: true, EnableRead: true}, nil); err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 1 {
		t.Fatalf("manual title conversation should not request title summary, got %d upstream calls", upstreamCalls)
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
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	var sawAssistant bool
	var sawDone bool
	var sawWarning bool
	service := NewService(st, box, "")
	msg, err := service.SendMessage(ctx, owner, conv.ID, SendRequest{Content: "Trigger title failure", EnableSearch: false, EnableRead: false}, func(event string, payload any) error {
		switch event {
		case "assistant_message":
			sawAssistant = true
		case "done":
			sawDone = true
		case "warning":
			sawWarning = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Answer before title failure." || !sawAssistant || !sawDone || !sawWarning {
		t.Fatalf("title failure should not block assistant reply: msg=%#v assistant=%v done=%v warning=%v", msg, sawAssistant, sawDone, sawWarning)
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
	conv, err := st.CreateChatConversation(ctx, owner, "Manual title", "gpt-test", "medium", "", "", "", "")
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
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "")
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
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "")
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
	busyConv, err := st.CreateChatConversation(ctx, owner, "busy", "gpt-test", "medium", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	freeConv, err := st.CreateChatConversation(ctx, owner, "free", "gpt-test", "medium", "", "", "", "")
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
	conv, err := st.CreateChatConversation(ctx, owner, "", "gpt-test", "medium", "", "", "", "")
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
