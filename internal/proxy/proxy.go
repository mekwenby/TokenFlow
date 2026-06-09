package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tokenflow/internal/auth"
	"tokenflow/internal/convert"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
)

const (
	ProtocolOpenAI    = "openai"
	ProtocolAnthropic = "anthropic"
)

type Handler struct {
	store  *store.Store
	box    *secret.Box
	client *http.Client
}

type requestContext struct {
	Key       store.DistributionKey
	Route     store.Route
	Protocol  string
	Model     string
	Stream    bool
	StartedAt time.Time
}

func New(st *store.Store, box *secret.Box) *Handler {
	return &Handler{
		store: st,
		box:   box,
		client: &http.Client{
			Timeout: 0,
		},
	}
}

func (h *Handler) OpenAIChat(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, ProtocolOpenAI)
}

func (h *Handler) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, ProtocolAnthropic)
}

func (h *Handler) OpenAIModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.models(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return
	}
	data := make([]any, 0, len(models))
	for _, model := range models {
		data = append(data, map[string]any{"id": model, "object": "model", "owned_by": "tokenflow"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *Handler) AnthropicModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.models(r.Context())
	if err != nil {
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	data := make([]any, 0, len(models))
	for _, model := range models {
		data = append(data, map[string]any{
			"id":           model,
			"object":       "model",
			"type":         "model",
			"display_name": model,
			"created":      "2025-01-01T00:00:00Z",
			"owned_by":     "tokenflow",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *Handler) handle(w http.ResponseWriter, r *http.Request, inbound string) {
	started := time.Now()
	ctx := r.Context()
	reqBody, err := decodeJSON(r.Body)
	if err != nil {
		h.writeProtocolError(w, inbound, http.StatusBadRequest, "invalid_request_error", "invalid JSON request body")
		return
	}

	keyValue := keyFromRequest(r, inbound)
	key, err := h.authenticate(ctx, keyValue)
	if err != nil {
		h.writeProtocolError(w, inbound, http.StatusUnauthorized, "authentication_error", "invalid or disabled API key")
		return
	}

	clientModel := convert.Model(reqBody)
	route, err := h.store.ResolveRoute(ctx, clientModel)
	if err != nil {
		msg := "no enabled upstream provider configured"
		if !errors.Is(err, store.ErrNotFound) {
			msg = err.Error()
		}
		h.writeProtocolError(w, inbound, http.StatusBadGateway, "api_error", msg)
		return
	}
	apiKey, err := h.box.Decrypt(route.Provider.APIKeyCipher)
	if err != nil {
		h.writeProtocolError(w, inbound, http.StatusBadGateway, "api_error", "failed to decrypt upstream API key")
		return
	}
	route.Provider.PlainAPIKey = apiKey
	if clientModel == "" {
		clientModel = route.UpstreamModel
	}

	reqCtx := requestContext{
		Key:       key,
		Route:     route,
		Protocol:  inbound,
		Model:     clientModel,
		Stream:    convert.IsStream(reqBody),
		StartedAt: started,
	}
	upstreamBody := h.upstreamRequestBody(inbound, route.Provider.Protocol, reqBody, route.UpstreamModel)
	if reqCtx.Stream && route.Provider.Protocol == ProtocolOpenAI {
		ensureOpenAIStreamUsage(upstreamBody)
	}

	upstreamResp, err := h.callUpstream(ctx, r, route.Provider, upstreamBody)
	if err != nil {
		h.finish(ctx, reqCtx, http.StatusBadGateway, convert.Usage{})
		h.writeProtocolError(w, inbound, http.StatusBadGateway, "api_error", err.Error())
		return
	}
	defer upstreamResp.Body.Close()

	if upstreamResp.StatusCode < 200 || upstreamResp.StatusCode >= 300 {
		body, _ := io.ReadAll(upstreamResp.Body)
		h.finish(ctx, reqCtx, upstreamResp.StatusCode, convert.Usage{})
		h.writeUpstreamError(w, inbound, upstreamResp.StatusCode, body)
		return
	}

	if reqCtx.Stream {
		usage, err := h.stream(w, inbound, route.Provider.Protocol, route.UpstreamModel, upstreamResp.Body)
		status := http.StatusOK
		if err != nil {
			status = http.StatusBadGateway
		}
		h.finish(ctx, reqCtx, status, usage)
		return
	}

	respBody, err := decodeJSON(upstreamResp.Body)
	if err != nil {
		h.finish(ctx, reqCtx, http.StatusBadGateway, convert.Usage{})
		h.writeProtocolError(w, inbound, http.StatusBadGateway, "api_error", "invalid upstream JSON response")
		return
	}

	var out map[string]any
	var usage convert.Usage
	switch {
	case inbound == route.Provider.Protocol:
		out = respBody
		if inbound == ProtocolOpenAI {
			usage = convert.UsageFromOpenAI(respBody)
		} else {
			usage = convert.UsageFromAnthropic(respBody)
		}
	case inbound == ProtocolAnthropic && route.Provider.Protocol == ProtocolOpenAI:
		out = convert.OpenAIResponseToAnthropic(respBody, route.UpstreamModel)
		usage = convert.UsageFromAnthropic(out)
	case inbound == ProtocolOpenAI && route.Provider.Protocol == ProtocolAnthropic:
		out = convert.AnthropicResponseToOpenAI(respBody, route.UpstreamModel)
		usage = convert.UsageFromOpenAI(out)
	}
	h.finish(ctx, reqCtx, http.StatusOK, usage)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) upstreamRequestBody(inbound, upstream string, req map[string]any, model string) map[string]any {
	switch {
	case inbound == upstream:
		copied := cloneJSON(req)
		copied["model"] = model
		return copied
	case inbound == ProtocolAnthropic && upstream == ProtocolOpenAI:
		return convert.AnthropicRequestToOpenAI(req, model)
	case inbound == ProtocolOpenAI && upstream == ProtocolAnthropic:
		return convert.OpenAIRequestToAnthropic(req, model)
	default:
		return cloneJSON(req)
	}
}

func (h *Handler) callUpstream(ctx context.Context, inbound *http.Request, provider store.Provider, body map[string]any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL(provider), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.Protocol == ProtocolOpenAI {
		req.Header.Set("Authorization", "Bearer "+provider.PlainAPIKey)
	} else {
		req.Header.Set("x-api-key", provider.PlainAPIKey)
		version := inbound.Header.Get("anthropic-version")
		if version == "" {
			version = "2023-06-01"
		}
		req.Header.Set("anthropic-version", version)
		if beta := inbound.Header.Get("anthropic-beta"); beta != "" {
			req.Header.Set("anthropic-beta", beta)
		}
	}
	return h.client.Do(req)
}

func (h *Handler) stream(w http.ResponseWriter, inbound, upstream, model string, body io.Reader) (convert.Usage, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}

	switch {
	case inbound == ProtocolOpenAI && upstream == ProtocolOpenAI:
		return streamOpenAIToOpenAI(w, flush, body)
	case inbound == ProtocolAnthropic && upstream == ProtocolAnthropic:
		return streamAnthropicToAnthropic(w, flush, body)
	case inbound == ProtocolAnthropic && upstream == ProtocolOpenAI:
		return streamOpenAIToAnthropic(w, flush, body, model)
	case inbound == ProtocolOpenAI && upstream == ProtocolAnthropic:
		return streamAnthropicToOpenAI(w, flush, body, model)
	default:
		return convert.Usage{}, fmt.Errorf("unsupported stream conversion")
	}
}

func streamOpenAIToOpenAI(w io.Writer, flush func(), body io.Reader) (convert.Usage, error) {
	var usage convert.Usage
	err := readSSE(body, func(event sseEvent) error {
		if event.Data == "" {
			return nil
		}
		if event.Data == "[DONE]" {
			_, err := w.Write([]byte("data: [DONE]\n\n"))
			flush()
			return err
		}
		var chunk map[string]any
		if json.Unmarshal([]byte(event.Data), &chunk) == nil {
			next := convert.UsageFromOpenAI(chunk)
			if next.HasAny() {
				usage = mergeUsage(usage, next)
			}
		}
		_, err := w.Write([]byte("data: " + event.Data + "\n\n"))
		flush()
		return err
	})
	return usage, err
}

func streamAnthropicToAnthropic(w io.Writer, flush func(), body io.Reader) (convert.Usage, error) {
	var usage convert.Usage
	err := readSSE(body, func(event sseEvent) error {
		if event.Data == "" {
			return nil
		}
		var payload map[string]any
		if json.Unmarshal([]byte(event.Data), &payload) == nil {
			if event.Event == "message_start" {
				if msg, ok := payload["message"].(map[string]any); ok {
					usage = mergeUsage(usage, convert.UsageFromAnthropic(msg))
				}
			}
			if event.Event == "message_delta" {
				usage = mergeUsage(usage, convert.UsageFromAnthropic(payload))
			}
		}
		name := event.Event
		if name == "" {
			name = stringField(payload, "type")
		}
		if name != "" {
			if _, err := w.Write([]byte("event: " + name + "\n")); err != nil {
				return err
			}
		}
		_, err := w.Write([]byte("data: " + event.Data + "\n\n"))
		flush()
		return err
	})
	return usage, err
}

func streamOpenAIToAnthropic(w io.Writer, flush func(), body io.Reader, model string) (convert.Usage, error) {
	messageID := "msg_" + strconv.FormatInt(time.Now().UnixNano(), 16)
	writeAndFlush(w, flush, convert.BuildAnthropicEvent("message_start", map[string]any{
		"message": map[string]any{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	}))

	var usage convert.Usage
	var textOpen bool
	var textIndex = -1
	var nextIndex int
	var finishReason = "end_turn"
	toolIndexes := map[int]int{}
	openToolBlocks := map[int]bool{}

	err := readSSE(body, func(event sseEvent) error {
		if event.Data == "" || event.Data == "[DONE]" {
			return nil
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			return nil
		}
		nextUsage := convert.UsageFromOpenAI(chunk)
		if nextUsage.HasAny() {
			usage = mergeUsage(usage, nextUsage)
		}
		for _, choiceValue := range arrayField(chunk, "choices") {
			choice, _ := choiceValue.(map[string]any)
			if reason := stringField(choice, "finish_reason"); reason != "" {
				finishReason = openAIFinishToAnthropic(reason)
			}
			delta, _ := choice["delta"].(map[string]any)
			if content := stringField(delta, "content"); content != "" {
				if !textOpen {
					textIndex = nextIndex
					nextIndex++
					textOpen = true
					writeAndFlush(w, flush, convert.BuildAnthropicEvent("content_block_start", map[string]any{
						"index":         textIndex,
						"content_block": map[string]any{"type": "text", "text": ""},
					}))
				}
				writeAndFlush(w, flush, convert.BuildAnthropicEvent("content_block_delta", map[string]any{
					"index": textIndex,
					"delta": map[string]any{"type": "text_delta", "text": content},
				}))
			}
			for _, toolValue := range arrayField(delta, "tool_calls") {
				tool, _ := toolValue.(map[string]any)
				upstreamIndex := intField(tool, "index", 0)
				blockIndex, ok := toolIndexes[upstreamIndex]
				fn, _ := tool["function"].(map[string]any)
				if !ok {
					blockIndex = nextIndex
					nextIndex++
					toolIndexes[upstreamIndex] = blockIndex
					openToolBlocks[blockIndex] = true
					id := stringField(tool, "id")
					if id == "" {
						id = "toolu_" + strconv.Itoa(upstreamIndex)
					}
					writeAndFlush(w, flush, convert.BuildAnthropicEvent("content_block_start", map[string]any{
						"index": blockIndex,
						"content_block": map[string]any{
							"type":  "tool_use",
							"id":    id,
							"name":  stringField(fn, "name"),
							"input": map[string]any{},
						},
					}))
				}
				if args := stringField(fn, "arguments"); args != "" {
					writeAndFlush(w, flush, convert.BuildAnthropicEvent("content_block_delta", map[string]any{
						"index": blockIndex,
						"delta": map[string]any{"type": "input_json_delta", "partial_json": args},
					}))
				}
			}
		}
		return nil
	})
	if err != nil {
		return usage, err
	}
	if textOpen {
		writeAndFlush(w, flush, convert.BuildAnthropicEvent("content_block_stop", map[string]any{"index": textIndex}))
	}
	for blockIndex := range openToolBlocks {
		writeAndFlush(w, flush, convert.BuildAnthropicEvent("content_block_stop", map[string]any{"index": blockIndex}))
	}
	writeAndFlush(w, flush, convert.BuildAnthropicEvent("message_delta", map[string]any{
		"delta": map[string]any{"stop_reason": finishReason, "stop_sequence": nil},
		"usage": map[string]any{
			"input_tokens":                usage.InputTokens,
			"output_tokens":               usage.OutputTokens,
			"cache_read_input_tokens":     usage.CacheReadTokens,
			"cache_creation_input_tokens": usage.CacheCreationTokens,
		},
	}))
	writeAndFlush(w, flush, convert.BuildAnthropicEvent("message_stop", map[string]any{}))
	return usage, nil
}

func streamAnthropicToOpenAI(w io.Writer, flush func(), body io.Reader, model string) (convert.Usage, error) {
	writeAndFlush(w, flush, convert.MarshalSSEData(convert.BuildOpenAIChunk(model, map[string]any{"role": "assistant"}, nil)))
	var usage convert.Usage
	var finishReason any
	err := readSSE(body, func(event sseEvent) error {
		if event.Data == "" {
			return nil
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.Data), &payload); err != nil {
			return nil
		}
		switch event.Event {
		case "message_start":
			if msg, ok := payload["message"].(map[string]any); ok {
				usage = mergeUsage(usage, convert.UsageFromAnthropic(msg))
			}
		case "content_block_start":
			block, _ := payload["content_block"].(map[string]any)
			if stringField(block, "type") == "tool_use" {
				index := intField(payload, "index", 0)
				chunk := convert.BuildOpenAIChunk(model, map[string]any{
					"tool_calls": []any{map[string]any{
						"index": index,
						"id":    stringField(block, "id"),
						"type":  "function",
						"function": map[string]any{
							"name":      stringField(block, "name"),
							"arguments": "",
						},
					}},
				}, nil)
				writeAndFlush(w, flush, convert.MarshalSSEData(chunk))
			}
		case "content_block_delta":
			delta, _ := payload["delta"].(map[string]any)
			index := intField(payload, "index", 0)
			switch stringField(delta, "type") {
			case "text_delta":
				writeAndFlush(w, flush, convert.MarshalSSEData(convert.BuildOpenAIChunk(model, map[string]any{"content": stringField(delta, "text")}, nil)))
			case "input_json_delta":
				chunk := convert.BuildOpenAIChunk(model, map[string]any{
					"tool_calls": []any{map[string]any{
						"index": index,
						"function": map[string]any{
							"arguments": stringField(delta, "partial_json"),
						},
					}},
				}, nil)
				writeAndFlush(w, flush, convert.MarshalSSEData(chunk))
			}
		case "message_delta":
			usage = mergeUsage(usage, convert.UsageFromAnthropic(payload))
			delta, _ := payload["delta"].(map[string]any)
			if reason := stringField(delta, "stop_reason"); reason != "" {
				finishReason = anthropicStopToOpenAI(reason)
			}
		case "error":
			writeAndFlush(w, flush, convert.MarshalSSEData(map[string]any{"error": payload["error"]}))
		}
		return nil
	})
	if err != nil {
		return usage, err
	}
	writeAndFlush(w, flush, convert.MarshalSSEData(convert.BuildOpenAIChunk(model, map[string]any{}, finishReason)))
	_, err = w.Write([]byte("data: [DONE]\n\n"))
	flush()
	return usage, err
}

func (h *Handler) authenticate(ctx context.Context, value string) (store.DistributionKey, error) {
	if value == "" {
		return store.DistributionKey{}, store.ErrNotFound
	}
	key, err := h.store.DistributionKeyByHash(ctx, auth.HashKey(value))
	if err != nil {
		return key, err
	}
	if !key.Enabled {
		return key, store.ErrNotFound
	}
	return key, nil
}

func (h *Handler) finish(ctx context.Context, req requestContext, statusCode int, usage convert.Usage) {
	providerID := req.Route.Provider.ID
	distributionKeyID := req.Key.ID
	_ = h.store.InsertRequestLog(ctx, store.RequestLog{
		Protocol:            req.Protocol,
		Model:               req.Model,
		ProviderID:          &providerID,
		DistributionKeyID:   &distributionKeyID,
		DistributionKeyName: req.Key.Name,
		StatusCode:          statusCode,
		LatencyMS:           time.Since(req.StartedAt).Milliseconds(),
		InputTokens:         usage.InputTokens,
		OutputTokens:        usage.OutputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		Stream:              req.Stream,
	})
	if statusCode >= 200 && statusCode < 300 {
		_ = h.store.UpdateKeyStats(ctx, req.Key.ID, usage.InputTokens, usage.CacheReadTokens, usage.OutputTokens)
	}
}

func (h *Handler) models(ctx context.Context) ([]string, error) {
	providers, err := h.store.Providers(ctx)
	if err != nil {
		return nil, err
	}
	mappings, err := h.store.Mappings(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var models []string
	for _, m := range mappings {
		if !seen[m.ClientModel] {
			models = append(models, m.ClientModel)
			seen[m.ClientModel] = true
		}
	}
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		for _, model := range p.Models {
			if model != "" && !seen[model] {
				models = append(models, model)
				seen[model] = true
			}
		}
	}
	return models, nil
}

func (h *Handler) writeProtocolError(w http.ResponseWriter, protocol string, status int, errType, message string) {
	if protocol == ProtocolAnthropic {
		writeAnthropicError(w, status, errType, message)
		return
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"type": errType, "message": message}})
}

func (h *Handler) writeUpstreamError(w http.ResponseWriter, protocol string, status int, body []byte) {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = "upstream error"
	}
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) == nil {
		if errObj, ok := parsed["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok {
				message = msg
			}
		}
	}
	h.writeProtocolError(w, protocol, status, "api_error", message)
}

func keyFromRequest(r *http.Request, protocol string) string {
	if protocol == ProtocolAnthropic {
		if key := strings.TrimSpace(r.Header.Get("x-api-key")); key != "" {
			return key
		}
	}
	return auth.ExtractBearer(r.Header.Get("Authorization"))
}

func upstreamURL(provider store.Provider) string {
	base := strings.TrimRight(provider.BaseAPI, "/")
	if provider.Protocol == ProtocolAnthropic {
		return base + "/messages"
	}
	return base + "/chat/completions"
}

func ensureOpenAIStreamUsage(req map[string]any) {
	options, _ := req["stream_options"].(map[string]any)
	if options == nil {
		options = map[string]any{}
	}
	options["include_usage"] = true
	req["stream_options"] = options
}

func decodeJSON(r io.Reader) (map[string]any, error) {
	var out map[string]any
	dec := json.NewDecoder(r)
	dec.UseNumber()
	err := dec.Decode(&out)
	return out, err
}

func cloneJSON(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAnthropicError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, map[string]any{"type": "error", "error": map[string]any{"type": errType, "message": message}})
}

func writeAndFlush(w io.Writer, flush func(), data []byte) {
	_, _ = w.Write(data)
	flush()
}

func mergeUsage(current, next convert.Usage) convert.Usage {
	if next.InputTokens > 0 {
		current.InputTokens = next.InputTokens
	}
	if next.OutputTokens > 0 {
		current.OutputTokens = next.OutputTokens
	}
	if next.CacheReadTokens > 0 {
		current.CacheReadTokens = next.CacheReadTokens
	}
	if next.CacheCreationTokens > 0 {
		current.CacheCreationTokens = next.CacheCreationTokens
	}
	return current
}

func arrayField(m map[string]any, key string) []any {
	if arr, ok := m[key].([]any); ok {
		return arr
	}
	return nil
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func intField(m map[string]any, key string, fallback int) int {
	if m == nil {
		return fallback
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
	}
	return fallback
}

func openAIFinishToAnthropic(reason string) string {
	switch reason {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func anthropicStopToOpenAI(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}
