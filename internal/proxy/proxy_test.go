package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tokenflow/internal/auth"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
)

func TestAnthropicToOpenAINonStreamingProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer upstream-key" {
			t.Fatalf("upstream auth was not rewritten")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "upstream-model" {
			t.Fatalf("unexpected upstream model: %#v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	}))
	defer upstream.Close()

	handler, clientKey := testHandler(t, upstream.URL+"/v1")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec := httptest.NewRecorder()

	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["type"] != "message" || body["stop_reason"] != "end_turn" {
		t.Fatalf("unexpected anthropic response: %#v", body)
	}
	content := body["content"].([]any)
	if content[0].(map[string]any)["text"] != "hello" {
		t.Fatalf("response was not converted: %#v", body)
	}
}

func TestNonStreamingProxyLogsCacheHitRate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"cached"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":4}}}`))
	}))
	defer upstream.Close()

	handler, st, clientKey := testHandlerWithStore(t, upstream.URL+"/v1")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec := httptest.NewRecorder()

	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	logs, err := st.Logs(context.Background(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].CacheReadTokens != 4 || logs[0].CacheHitRate < 0.39 || logs[0].CacheHitRate > 0.41 || logs[0].DistributionKeyName != "test" {
		t.Fatalf("unexpected cache log: %#v", logs)
	}
	keys, err := st.DistributionKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].InputTokens != 10 || keys[0].CacheReadTokens != 4 || keys[0].OutputTokens != 2 {
		t.Fatalf("unexpected key stats: %#v", keys)
	}
}

func TestOpenAIModelsIncludesMappingsAndProviderModels(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	handler, _ := testHandler(t, upstream.URL+"/v1")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	handler.OpenAIModels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range body["data"].([]any) {
		model := item.(map[string]any)["id"].(string)
		seen[model] = true
	}
	for _, expected := range []string{"client-model", "fallback-model", "upstream-model", "extra-model"} {
		if !seen[expected] {
			t.Fatalf("missing model %q in %#v", expected, body)
		}
	}
}

func TestAnthropicToOpenAIStreamingProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	handler, clientKey := testHandler(t, upstream.URL+"/v1")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","stream":true,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec := httptest.NewRecorder()

	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	for _, expected := range []string{"event: message_start", "event: content_block_start", "event: content_block_delta", `"text":"Hel"`, `"text":"lo"`, "event: message_delta", "event: message_stop"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, out)
		}
	}
}

func TestAnthropicToOpenAIStreamingProxyConvertsReasoningContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"reasoning_content":"I should use the tool context."},"finish_reason":null}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"content":"Done"},"finish_reason":"stop"}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":3}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	handler, clientKey := testHandler(t, upstream.URL+"/v1")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","stream":true,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec := httptest.NewRecorder()

	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	for _, expected := range []string{`"type":"thinking"`, `"type":"thinking_delta"`, `"thinking":"I should use the tool context."`, `"type":"text_delta"`, `"text":"Done"`} {
		if !strings.Contains(out, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, out)
		}
	}
}

func TestConsumerKeyQuotaEnforced(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	}))
	defer upstream.Close()

	handler, st, _ := testHandlerWithStore(t, upstream.URL+"/v1")
	ctx := context.Background()
	user, err := st.CreateConsumerUser(ctx, "customer@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "sk-consumer-client"
	if _, err := st.CreateConsumerDistributionKey(ctx, user.ID, "customer", "sk-consumer", auth.HashKey(clientKey)); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec := httptest.NewRecorder()
	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusUnauthorized || upstreamCalls != 0 {
		t.Fatalf("pending user should be rejected before upstream: status=%d calls=%d body=%s", rec.Code, upstreamCalls, rec.Body.String())
	}

	if _, err := st.UpdateConsumerUser(ctx, user.ID, store.ConsumerStatusEnabled, 5); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec = httptest.NewRecorder()
	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("enabled user should succeed: %d %s", rec.Code, rec.Body.String())
	}
	user, err = st.ConsumerUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user.QuotaUsedTokens != 5 || user.RequestCount != 1 {
		t.Fatalf("quota was not consumed: %#v", user)
	}
	logs, err := st.Logs(ctx, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ConsumerEmail != "customer@example.com" || logs[0].ConsumerUserID == nil {
		t.Fatalf("consumer log was not recorded: %#v", logs)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec = httptest.NewRecorder()
	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusTooManyRequests || upstreamCalls != 1 {
		t.Fatalf("exhausted user should be rejected before upstream: status=%d calls=%d body=%s", rec.Code, upstreamCalls, rec.Body.String())
	}
}

func TestConsumerStreamingRequestUpdatesQuota(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":"stop"}],"usage":null}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	handler, st, _ := testHandlerWithStore(t, upstream.URL+"/v1")
	ctx := context.Background()
	user, err := st.CreateConsumerUser(ctx, "stream@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateConsumerUser(ctx, user.ID, store.ConsumerStatusEnabled, 10); err != nil {
		t.Fatal(err)
	}
	clientKey := "sk-stream-client"
	if _, err := st.CreateConsumerDistributionKey(ctx, user.ID, "stream", "sk-stream", auth.HashKey(clientKey)); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"client-model","stream":true,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("x-api-key", clientKey)
	rec := httptest.NewRecorder()
	handler.AnthropicMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected stream status %d: %s", rec.Code, rec.Body.String())
	}
	user, err = st.ConsumerUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user.QuotaUsedTokens != 7 || user.RequestCount != 1 {
		t.Fatalf("stream usage was not consumed: %#v", user)
	}
}

func testHandler(t *testing.T, baseAPI string) (*Handler, string) {
	handler, _, clientKey := testHandlerWithStore(t, baseAPI)
	return handler, clientKey
}

func testHandlerWithStore(t *testing.T, baseAPI string) (*Handler, *store.Store, string) {
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
	t.Cleanup(func() { _ = st.Close() })
	cipher, err := box.Encrypt("upstream-key")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := st.CreateProvider(ctx, store.ProviderInput{
		Name:         "openai",
		Protocol:     "openai",
		BaseAPI:      baseAPI,
		APIKeyCipher: cipher,
		DefaultModel: "fallback-model",
		Models:       []string{"upstream-model", "extra-model"},
		Enabled:      true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMapping(ctx, "client-model", provider.ID, "upstream-model"); err != nil {
		t.Fatal(err)
	}
	clientKey := "sk-test-client"
	if _, err := st.CreateDistributionKey(ctx, "test", "sk-test", auth.HashKey(clientKey)); err != nil {
		t.Fatal(err)
	}
	return New(st, box), st, clientKey
}
