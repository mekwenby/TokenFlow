package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tokenflow/internal/convert"
	"tokenflow/internal/secret"
	"tokenflow/internal/store"
)

const (
	defaultInfoFlowBaseURL = "https://infoflow.030399.xyz"
	maxToolResultChars     = 20000
	titleSummaryMaxTokens  = 64
	titleSummaryTimeout    = 45 * time.Second
	responseLockTimeout    = 30 * time.Minute
	titleLockTimeout       = 5 * time.Minute
)

var (
	ErrEmptyMessage    = errors.New("message content is required")
	ErrNoModel         = errors.New("no chat model is available")
	ErrNoTitleMessages = errors.New("current conversation has no messages to summarize")
	ErrEmptyTitle      = errors.New("title generation returned an empty title")
)

type Service struct {
	store           *store.Store
	box             *secret.Box
	infoFlowBaseURL string
	client          *http.Client
	infoFlowClient  *http.Client
}

type SendRequest struct {
	Content        string `json:"content"`
	Model          string `json:"model"`
	ThinkingEffort string `json:"thinking_effort"`
	EnableSearch   bool   `json:"enable_search"`
	EnableRead     bool   `json:"enable_read"`
	ShowProcess    bool   `json:"show_process"`
}

type Emitter func(event string, payload any) error

type responsePayload struct {
	body     map[string]any
	usage    convert.Usage
	status   int
	warning  string
	streamed bool
}

type toolCall struct {
	ID        string
	Name      string
	Arguments string
	OpenAI    map[string]any
}

func NewService(st *store.Store, box *secret.Box, infoFlowBaseURL string) *Service {
	infoFlowBaseURL = strings.TrimRight(strings.TrimSpace(infoFlowBaseURL), "/")
	if infoFlowBaseURL == "" {
		infoFlowBaseURL = defaultInfoFlowBaseURL
	}
	return &Service{
		store:           st,
		box:             box,
		infoFlowBaseURL: infoFlowBaseURL,
		client:          &http.Client{Timeout: 0},
		infoFlowClient:  &http.Client{Timeout: 45 * time.Second},
	}
}

func (s *Service) Models(ctx context.Context) ([]string, error) {
	return s.store.AvailableModels(ctx)
}

func (s *Service) DefaultSystemPrompt() string {
	return defaultSystemPrompt()
}

func (s *Service) CanChat(ctx context.Context, owner store.ChatOwner) error {
	return s.ensureOwnerCanChat(ctx, owner)
}

func titleSummaryMessages(transcript string) []any {
	return []any{
		map[string]any{
			"role":    "system",
			"content": "Write a concise chat title. Return only the title, without quotes, numbering, markdown, or explanation. Use the user's language when it is clear. Keep it under 12 words.",
		},
		map[string]any{
			"role":    "user",
			"content": "Conversation transcript:\n" + transcript,
		},
	}
}

func (s *Service) summarizeTitle(ctx context.Context, owner store.ChatOwner, model, transcript, fallback string) (string, error) {
	messages := titleSummaryMessages(transcript)
	resp, err := s.callModel(ctx, owner, model, "off", messages, nil, nil, titleSummaryMaxTokens)
	if err != nil {
		return "", err
	}
	if title := cleanGeneratedTitle(messageContent(firstOpenAIMessage(resp.body))); title != "" {
		return title, nil
	}
	resp, err = s.callModelNonStreaming(ctx, owner, model, "off", messages, nil, titleSummaryMaxTokens)
	if err != nil {
		return "", err
	}
	if title := cleanGeneratedTitle(messageContent(firstOpenAIMessage(resp.body))); title != "" {
		return title, nil
	}
	if fallback = cleanGeneratedTitle(fallback); fallback != "" {
		return fallback, nil
	}
	return "", ErrEmptyTitle
}

func (s *Service) callModelNonStreaming(ctx context.Context, owner store.ChatOwner, model, thinkingEffort string, messages, tools []any, maxTokens int) (responsePayload, error) {
	route, err := s.store.ResolveRoute(ctx, model)
	if err != nil {
		return responsePayload{}, err
	}
	apiKey, err := s.box.Decrypt(route.Provider.APIKeyCipher)
	if err != nil {
		return responsePayload{}, err
	}
	route.Provider.PlainAPIKey = apiKey
	clientModel := model
	if clientModel == "" {
		clientModel = route.UpstreamModel
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body := map[string]any{
		"model":      route.UpstreamModel,
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     false,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	withThinking := applyThinking(body, route.Provider.Protocol, thinkingEffort)
	resp, err := s.performModelRequest(ctx, owner, route, clientModel, body)
	if err == nil {
		return resp, nil
	}
	if withThinking && retryWithoutThinking(resp.status) {
		retried, retryErr := s.performModelRequest(ctx, owner, route, clientModel, removeThinking(body, route.Provider.Protocol))
		retried.warning = "The upstream provider rejected thinking parameters, so TokenFlow retried without them."
		if retryErr != nil {
			return retried, retryErr
		}
		return retried, nil
	}
	return resp, err
}

func (s *Service) GenerateConversationTitle(ctx context.Context, owner store.ChatOwner, conversationID int64, force bool) (store.ChatConversation, error) {
	return s.generateConversationTitle(ctx, owner, conversationID, force, nil)
}

func (s *Service) generateConversationTitle(ctx context.Context, owner store.ChatOwner, conversationID int64, force bool, emit Emitter) (conv store.ChatConversation, err error) {
	if err := s.ensureOwnerCanChat(ctx, owner); err != nil {
		return store.ChatConversation{}, err
	}
	conv, err = s.store.ChatConversation(ctx, owner, conversationID)
	if err != nil {
		return store.ChatConversation{}, err
	}
	if !force && !conv.TitleAutoGenerated {
		return conv, nil
	}
	conv, err = s.store.StartChatConversationOperation(ctx, owner, conversationID, store.ChatConversationOperationTitleGenerating, "", titleLockTimeout)
	if err != nil {
		return store.ChatConversation{}, err
	}
	_ = emitJSON(emit, "conversation", conv)
	operationActive := true
	finish := func(status, message string) {
		if !operationActive {
			return
		}
		if updated, finishErr := s.store.FinishChatConversationOperation(context.Background(), owner, conversationID, store.ChatConversationOperationTitleGenerating, status, message); finishErr == nil {
			conv = updated
			_ = emitJSON(emit, "conversation", conv)
		}
		operationActive = false
	}
	defer func() {
		if !operationActive {
			return
		}
		status := store.ChatConversationStatusFailed
		message := ""
		if errors.Is(ctx.Err(), context.Canceled) {
			status = store.ChatConversationStatusStopped
			message = "stopped"
		} else if err != nil {
			message = err.Error()
		}
		finish(status, message)
	}()

	history, err := s.store.ChatMessages(ctx, owner, conversationID)
	if err != nil {
		return store.ChatConversation{}, err
	}
	transcript := titleTranscript(history)
	if transcript == "" {
		finish(store.ChatConversationStatusIdle, "")
		return store.ChatConversation{}, ErrNoTitleMessages
	}
	model, err := s.resolveModel(ctx, conv.Model, "")
	if err != nil {
		return store.ChatConversation{}, err
	}

	title, err := s.summarizeTitle(ctx, owner, model, transcript, fallbackTitleFromMessages(history))
	if err != nil {
		return store.ChatConversation{}, err
	}
	conv, err = s.store.UpdateChatConversationGeneratedTitle(ctx, owner, conversationID, title, force)
	if err != nil {
		return store.ChatConversation{}, err
	}
	finish(store.ChatConversationStatusIdle, "")
	return conv, nil
}

func (s *Service) SendMessage(ctx context.Context, owner store.ChatOwner, conversationID int64, req SendRequest, emit Emitter) (assistantMessage store.ChatMessage, err error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return store.ChatMessage{}, ErrEmptyMessage
	}
	if err := s.ensureOwnerCanChat(ctx, owner); err != nil {
		return store.ChatMessage{}, err
	}

	conv, err := s.store.ChatConversation(ctx, owner, conversationID)
	if err != nil {
		return store.ChatMessage{}, err
	}
	model, err := s.resolveModel(ctx, conv.Model, "")
	if err != nil {
		return store.ChatMessage{}, err
	}
	thinkingEffort := store.NormalizeChatThinkingEffort(conv.ThinkingEffort)
	if conv.Model != model || conv.ThinkingEffort != thinkingEffort {
		conv, err = s.store.UpdateChatConversation(ctx, owner, conversationID, nil, &model, &thinkingEffort, nil, nil, nil, nil)
		if err != nil {
			return store.ChatMessage{}, err
		}
	}

	conv, err = s.store.StartChatConversationOperation(ctx, owner, conversationID, store.ChatConversationOperationResponding, "", responseLockTimeout)
	if err != nil {
		return store.ChatMessage{}, err
	}
	operationActive := true
	finishOperation := func(status, message string) {
		if !operationActive {
			return
		}
		if updated, finishErr := s.store.FinishChatConversationOperation(context.Background(), owner, conversationID, store.ChatConversationOperationResponding, status, message); finishErr == nil {
			conv = updated
			_ = emitJSON(emit, "conversation", conv)
		}
		operationActive = false
	}
	defer func() {
		if !operationActive {
			return
		}
		status := store.ChatConversationStatusFailed
		message := ""
		if errors.Is(ctx.Err(), context.Canceled) {
			status = store.ChatConversationStatusStopped
			message = "stopped"
		} else if err != nil {
			message = err.Error()
		}
		finishOperation(status, message)
	}()
	_ = emitJSON(emit, "conversation", conv)

	history, err := s.store.ChatMessages(ctx, owner, conversationID)
	if err != nil {
		return store.ChatMessage{}, err
	}
	shouldSummarizeTitle := shouldSummarizeConversationTitle(conv, history)

	userMessage, err := s.store.CreateChatMessage(ctx, owner, conversationID, store.ChatRoleUser, content, "{}")
	if err != nil {
		return store.ChatMessage{}, err
	}
	_ = emitJSON(emit, "user_message", userMessage)
	history = append(history, userMessage)

	maxToolCalls, err := s.store.ChatMaxToolCalls(ctx)
	if err != nil {
		return store.ChatMessage{}, err
	}
	messages := buildOpenAIMessages(history, conv.SystemPrompt, conv.Nickname)
	var tools []any
	if maxToolCalls > 0 {
		tools = buildTools(req.EnableSearch, req.EnableRead)
	}
	var usage convert.Usage
	var events []map[string]any
	finalContent := ""
	toolCallsUsed := 0
	var visibleContent strings.Builder
	emitDelta := func(content string) error {
		if content == "" {
			return nil
		}
		visibleContent.WriteString(content)
		return emitJSON(emit, "delta", map[string]any{"content": content})
	}

	for {
		if err := s.ensureOwnerCanChat(ctx, owner); err != nil {
			return store.ChatMessage{}, err
		}
		activeTools := tools
		if toolCallsUsed >= maxToolCalls {
			activeTools = nil
		}
		_ = emitJSON(emit, "status", map[string]any{"message": "Calling model", "message_key": "calling_model", "model": model})
		resp, err := s.callModel(ctx, owner, model, thinkingEffort, messages, activeTools, emitDelta, 4096)
		if resp.warning != "" {
			event := map[string]any{"type": "warning", "message": resp.warning, "created_at": time.Now().UTC()}
			events = append(events, event)
			_ = emitJSON(emit, "warning", event)
		}
		if err != nil {
			return store.ChatMessage{}, err
		}
		usage = mergeUsage(usage, resp.usage)

		message := firstOpenAIMessage(resp.body)
		if reasoning := reasoningSummary(message); reasoning != "" {
			event := map[string]any{"type": "thinking", "content": reasoning, "created_at": time.Now().UTC()}
			events = append(events, event)
			_ = emitJSON(emit, "thinking", event)
		}

		calls := extractToolCalls(message)
		if len(calls) == 0 {
			if !resp.streamed {
				_ = emitDelta(messageContent(message))
			}
			finalContent = visibleContent.String()
			break
		}
		if len(activeTools) == 0 {
			warning := toolLimitMessage(maxToolCalls)
			event := map[string]any{"type": "warning", "message": warning, "created_at": time.Now().UTC()}
			events = append(events, event)
			_ = emitJSON(emit, "warning", event)
			finalContent = warning
			_ = emitDelta(finalContent)
			break
		}

		assistantToolMessage := map[string]any{
			"role":       "assistant",
			"content":    messageContent(message),
			"tool_calls": normalizedToolCalls(calls),
		}
		messages = append(messages, assistantToolMessage)
		for _, call := range calls {
			if toolCallsUsed >= maxToolCalls {
				warning := toolLimitMessage(maxToolCalls)
				event := map[string]any{"type": "warning", "message": warning, "created_at": time.Now().UTC()}
				events = append(events, event)
				_ = emitJSON(emit, "warning", event)
				result := toolError(warning)
				done := map[string]any{
					"type":       "tool_result",
					"id":         call.ID,
					"name":       call.Name,
					"ok":         false,
					"result":     result,
					"created_at": time.Now().UTC(),
				}
				events = append(events, done)
				_ = emitJSON(emit, "tool_result", done)
				messages = append(messages, map[string]any{
					"role":         "tool",
					"tool_call_id": call.ID,
					"content":      result,
				})
				continue
			}
			start := map[string]any{
				"type":       "tool_start",
				"id":         call.ID,
				"name":       call.Name,
				"arguments":  call.Arguments,
				"created_at": time.Now().UTC(),
			}
			events = append(events, start)
			_ = emitJSON(emit, "tool_start", start)

			result, ok := s.executeTool(ctx, req, call)
			toolCallsUsed++
			done := map[string]any{
				"type":       "tool_result",
				"id":         call.ID,
				"name":       call.Name,
				"ok":         ok,
				"result":     result,
				"created_at": time.Now().UTC(),
			}
			events = append(events, done)
			_ = emitJSON(emit, "tool_result", done)
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": call.ID,
				"content":      result,
			})
		}
	}

	finalContent = strings.TrimSpace(finalContent)
	if finalContent == "" {
		finalContent = "The model returned an empty response."
		_ = emitDelta(finalContent)
	}
	metadata := map[string]any{
		"events":          events,
		"model":           model,
		"thinking_effort": thinkingEffort,
		"usage": map[string]any{
			"input_tokens":          usage.InputTokens,
			"output_tokens":         usage.OutputTokens,
			"cache_read_tokens":     usage.CacheReadTokens,
			"cache_creation_tokens": usage.CacheCreationTokens,
		},
	}
	rawMetadata, _ := json.Marshal(metadata)
	assistantMessage, err = s.store.CreateChatMessage(ctx, owner, conversationID, store.ChatRoleAssistant, finalContent, string(rawMetadata))
	if err != nil {
		return store.ChatMessage{}, err
	}
	_ = emitJSON(emit, "assistant_message", assistantMessage)
	_ = emitJSON(emit, "done", map[string]any{"message": assistantMessage, "usage": metadata["usage"]})
	finishOperation(store.ChatConversationStatusIdle, "")

	if shouldSummarizeTitle {
		titleCtx, cancel := context.WithTimeout(context.Background(), titleSummaryTimeout)
		defer cancel()
		if updated, err := s.generateConversationTitle(titleCtx, owner, conversationID, false, emit); err != nil {
			event := map[string]any{"type": "warning", "message": "TokenFlow could not summarize the conversation title: " + err.Error(), "created_at": time.Now().UTC()}
			_ = emitJSON(emit, "warning", event)
		} else {
			conv = updated
			_ = emitJSON(emit, "conversation", conv)
		}
	}
	return assistantMessage, nil
}

func (s *Service) ensureOwnerCanChat(ctx context.Context, owner store.ChatOwner) error {
	switch owner.Type {
	case store.ChatOwnerAdmin:
		_, err := s.store.AdminUser(ctx, owner.ID)
		return err
	case store.ChatOwnerConsumer:
		user, err := s.store.ConsumerUser(ctx, owner.ID)
		if err != nil {
			return err
		}
		if user.Status != store.ConsumerStatusEnabled {
			return store.ErrNotFound
		}
		if user.QuotaUsedTokens >= user.QuotaTotalTokens {
			return store.ErrQuotaExceeded
		}
		return nil
	default:
		return store.ErrNotFound
	}
}

func (s *Service) resolveModel(ctx context.Context, requested, conversationModel string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	if conversationModel != "" {
		return conversationModel, nil
	}
	models, err := s.Models(ctx)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", ErrNoModel
	}
	return models[0], nil
}

func (s *Service) callModel(ctx context.Context, owner store.ChatOwner, model, thinkingEffort string, messages, tools []any, emitDelta func(string) error, maxTokens int) (responsePayload, error) {
	route, err := s.store.ResolveRoute(ctx, model)
	if err != nil {
		return responsePayload{}, err
	}
	apiKey, err := s.box.Decrypt(route.Provider.APIKeyCipher)
	if err != nil {
		return responsePayload{}, err
	}
	route.Provider.PlainAPIKey = apiKey
	clientModel := model
	if clientModel == "" {
		clientModel = route.UpstreamModel
	}

	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body := map[string]any{
		"model":      route.UpstreamModel,
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     true,
	}
	if route.Provider.Protocol != "anthropic" {
		ensureOpenAIStreamUsage(body)
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	withThinking := applyThinking(body, route.Provider.Protocol, thinkingEffort)
	resp, err := s.performModelStream(ctx, owner, route, clientModel, body, emitDelta)
	if err == nil {
		return resp, nil
	}
	if withThinking && retryWithoutThinking(resp.status) {
		body = removeThinking(body, route.Provider.Protocol)
		retried, retryErr := s.performModelStream(ctx, owner, route, clientModel, body, emitDelta)
		retried.warning = "The upstream provider rejected thinking parameters, so TokenFlow retried without them."
		if retryErr == nil {
			return retried, nil
		}
		if retryWithoutStreaming(retried.status) {
			fallbackBody := removeStreaming(body)
			fallback, fallbackErr := s.performModelRequest(ctx, owner, route, clientModel, fallbackBody)
			fallback.warning = combineWarnings(retried.warning, "The upstream provider rejected streaming, so TokenFlow retried without streaming.")
			if fallbackErr == nil {
				return fallback, nil
			}
			return fallback, fallbackErr
		}
		return retried, retryErr
	}
	if retryWithoutStreaming(resp.status) {
		fallbackBody := removeStreaming(body)
		fallback, fallbackErr := s.performModelRequest(ctx, owner, route, clientModel, fallbackBody)
		fallback.warning = "The upstream provider rejected streaming, so TokenFlow retried without streaming."
		if fallbackErr != nil {
			if withThinking && retryWithoutThinking(fallback.status) {
				retryBody := removeThinking(fallbackBody, route.Provider.Protocol)
				retried, retryErr := s.performModelRequest(ctx, owner, route, clientModel, retryBody)
				retried.warning = combineWarnings(fallback.warning, "The upstream provider rejected thinking parameters, so TokenFlow retried without them.")
				if retryErr != nil {
					return retried, retryErr
				}
				return retried, nil
			}
			return fallback, fallbackErr
		}
		return fallback, nil
	}
	return resp, err
}

func (s *Service) performModelStream(ctx context.Context, owner store.ChatOwner, route store.Route, clientModel string, body map[string]any, emitDelta func(string) error) (responsePayload, error) {
	started := time.Now()
	upstreamBody := body
	if route.Provider.Protocol == "anthropic" {
		upstreamBody = convert.OpenAIRequestToAnthropic(body, route.UpstreamModel)
		if thinking, ok := body["thinking"]; ok {
			upstreamBody["thinking"] = thinking
		}
	}
	payload, err := json.Marshal(upstreamBody)
	if err != nil {
		return responsePayload{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL(route.Provider), bytes.NewReader(payload))
	if err != nil {
		return responsePayload{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if route.Provider.Protocol == "anthropic" {
		req.Header.Set("x-api-key", route.Provider.PlainAPIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+route.Provider.PlainAPIKey)
	}

	upstreamResp, err := s.client.Do(req)
	if err != nil {
		s.recordRequest(ctx, owner, route, clientModel, http.StatusBadGateway, time.Since(started), convert.Usage{})
		return responsePayload{status: http.StatusBadGateway, streamed: true}, err
	}
	defer upstreamResp.Body.Close()

	status := upstreamResp.StatusCode
	if status < 200 || status >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(upstreamResp.Body, 4<<20))
		s.recordRequest(ctx, owner, route, clientModel, status, time.Since(started), convert.Usage{})
		return responsePayload{status: status, streamed: true}, fmt.Errorf("upstream error: %s", upstreamErrorMessage(raw))
	}

	var parsed map[string]any
	var usage convert.Usage
	if route.Provider.Protocol == "anthropic" {
		parsed, usage, err = parseAnthropicStream(upstreamResp.Body, route.UpstreamModel, emitDelta)
	} else {
		parsed, usage, err = parseOpenAIStream(upstreamResp.Body, route.UpstreamModel, emitDelta)
	}
	if err != nil {
		s.recordRequest(ctx, owner, route, clientModel, http.StatusBadGateway, time.Since(started), usage)
		return responsePayload{status: http.StatusBadGateway, usage: usage, streamed: true}, err
	}
	s.recordRequest(ctx, owner, route, clientModel, status, time.Since(started), usage)
	return responsePayload{body: parsed, usage: usage, status: status, streamed: true}, nil
}

func (s *Service) performModelRequest(ctx context.Context, owner store.ChatOwner, route store.Route, clientModel string, body map[string]any) (responsePayload, error) {
	started := time.Now()
	upstreamBody := body
	if route.Provider.Protocol == "anthropic" {
		upstreamBody = convert.OpenAIRequestToAnthropic(body, route.UpstreamModel)
		if thinking, ok := body["thinking"]; ok {
			upstreamBody["thinking"] = thinking
		}
	}
	payload, err := json.Marshal(upstreamBody)
	if err != nil {
		return responsePayload{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL(route.Provider), bytes.NewReader(payload))
	if err != nil {
		return responsePayload{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if route.Provider.Protocol == "anthropic" {
		req.Header.Set("x-api-key", route.Provider.PlainAPIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+route.Provider.PlainAPIKey)
	}

	upstreamResp, err := s.client.Do(req)
	if err != nil {
		s.recordRequest(ctx, owner, route, clientModel, http.StatusBadGateway, time.Since(started), convert.Usage{})
		return responsePayload{status: http.StatusBadGateway}, err
	}
	defer upstreamResp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(upstreamResp.Body, 4<<20))
	status := upstreamResp.StatusCode
	if status < 200 || status >= 300 {
		s.recordRequest(ctx, owner, route, clientModel, status, time.Since(started), convert.Usage{})
		return responsePayload{status: status}, fmt.Errorf("upstream error: %s", upstreamErrorMessage(raw))
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		s.recordRequest(ctx, owner, route, clientModel, http.StatusBadGateway, time.Since(started), convert.Usage{})
		return responsePayload{status: http.StatusBadGateway}, err
	}
	usage := convert.UsageFromOpenAI(parsed)
	if route.Provider.Protocol == "anthropic" {
		reasoning := anthropicThinking(parsed)
		usage = convert.UsageFromAnthropic(parsed)
		parsed = convert.AnthropicResponseToOpenAI(parsed, route.UpstreamModel)
		if reasoning != "" {
			firstOpenAIMessage(parsed)["reasoning_content"] = reasoning
		}
	}
	s.recordRequest(ctx, owner, route, clientModel, status, time.Since(started), usage)
	return responsePayload{body: parsed, usage: usage, status: status}, nil
}

func (s *Service) recordRequest(ctx context.Context, owner store.ChatOwner, route store.Route, clientModel string, status int, latency time.Duration, usage convert.Usage) {
	providerID := route.Provider.ID
	log := store.RequestLog{
		Protocol:            "chat",
		Model:               clientModel,
		UpstreamModel:       route.UpstreamModel,
		ProviderID:          &providerID,
		StatusCode:          status,
		LatencyMS:           latency.Milliseconds(),
		InputTokens:         usage.InputTokens,
		OutputTokens:        usage.OutputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
	}
	switch owner.Type {
	case store.ChatOwnerConsumer:
		id := owner.ID
		log.ConsumerUserID = &id
		log.ConsumerEmail = owner.Name
	case store.ChatOwnerAdmin:
		id := owner.ID
		log.AdminUserID = &id
		log.AdminUsername = owner.Name
	}
	_ = s.store.RecordRequest(ctx, log)
}

func (s *Service) executeTool(ctx context.Context, req SendRequest, call toolCall) (string, bool) {
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return toolError("invalid tool arguments: " + err.Error()), false
	}
	switch call.Name {
	case "web_search":
		if !req.EnableSearch {
			return toolError("web search is disabled for this message"), false
		}
		return s.webSearch(ctx, args)
	case "read_url":
		if !req.EnableRead {
			return toolError("web page reading is disabled for this message"), false
		}
		return s.readURL(ctx, args)
	default:
		return toolError("unknown tool: " + call.Name), false
	}
}

func (s *Service) webSearch(ctx context.Context, args map[string]any) (string, bool) {
	query := strings.TrimSpace(stringArg(args, "query", ""))
	if query == "" {
		return toolError("query is required"), false
	}
	payload := map[string]any{
		"query":       query,
		"count":       clampInt(intArg(args, "count", 5), 1, 10),
		"language":    stringArg(args, "language", ""),
		"safe_search": stringArg(args, "safe_search", "moderate"),
	}
	return s.postInfoFlow(ctx, "/v1/web_search", payload)
}

func (s *Service) readURL(ctx context.Context, args map[string]any) (string, bool) {
	rawURL := strings.TrimSpace(stringArg(args, "url", ""))
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return toolError("valid http or https url is required"), false
	}
	payload := map[string]any{
		"url":        rawURL,
		"render":     boolArg(args, "render", true),
		"max_chars":  clampInt(intArg(args, "max_chars", 12000), 1000, maxToolResultChars),
		"wait_until": waitUntilArg(args, "wait_until"),
	}
	return s.postInfoFlow(ctx, "/v1/read_url", payload)
}

func (s *Service) postInfoFlow(ctx context.Context, path string, payload map[string]any) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	raw, err := json.Marshal(payload)
	if err != nil {
		return toolError(err.Error()), false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.infoFlowBaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return toolError(err.Error()), false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.infoFlowClient.Do(req)
	if err != nil {
		return toolError(err.Error()), false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return toolError(fmt.Sprintf("InfoFlow returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))), false
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return truncateToolResult(string(body)), true
	}
	encoded, _ := json.Marshal(parsed)
	return truncateToolResult(string(encoded)), true
}

func buildOpenAIMessages(history []store.ChatMessage, systemPrompt, nickname string) []any {
	messages := []any{map[string]any{"role": "system", "content": chatSystemPrompt(systemPrompt, nickname)}}
	for _, msg := range history {
		if msg.Role != store.ChatRoleUser && msg.Role != store.ChatRoleAssistant {
			continue
		}
		messages = append(messages, map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return messages
}

func chatSystemPrompt(systemPrompt, nickname string) string {
	parts := []string{defaultSystemPrompt()}
	if custom := store.NormalizeChatSystemPrompt(systemPrompt); custom != "" {
		parts = append(parts, "User custom system instructions:\n"+custom)
	}
	if name := store.NormalizeChatNickname(nickname); name != "" {
		parts = append(parts, "The user's preferred display name is: "+name)
	}
	return strings.Join(parts, "\n\n")
}

func buildTools(enableSearch, enableRead bool) []any {
	tools := make([]any, 0, 2)
	if enableSearch {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "web_search",
				"description": "Search the live web for current or source-sensitive information.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query":       map[string]any{"type": "string"},
						"count":       map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
						"language":    map[string]any{"type": "string"},
						"safe_search": map[string]any{"type": "string", "enum": []string{"off", "moderate", "strict"}},
					},
					"required": []string{"query"},
				},
			},
		})
	}
	if enableRead {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "read_url",
				"description": "Read a URL before relying on the page as a source.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"url":        map[string]any{"type": "string"},
						"render":     map[string]any{"type": "boolean"},
						"max_chars":  map[string]any{"type": "integer", "minimum": 1000, "maximum": maxToolResultChars},
						"wait_until": map[string]any{"type": "string", "enum": []string{"load", "domcontentloaded", "networkidle"}},
					},
					"required": []string{"url"},
				},
			},
		})
	}
	return tools
}

func defaultSystemPrompt() string {
	return strings.TrimSpace(`You are TokenFlow's web chat assistant.

Use the available web_search tool for current events, fast-changing facts, niche facts, or source-sensitive claims. Use read_url before relying on a search result or user-provided URL. Cite source URLs in the final answer when tools were used. Answer in the user's language unless they ask otherwise.

Do not claim access to hidden chain-of-thought. If the provider supplies a visible reasoning summary, keep it concise and separate from the final answer. Prefer direct, useful answers and explain uncertainty when sources are incomplete.`)
}

func applyThinking(body map[string]any, protocol, effort string) bool {
	effort = store.NormalizeChatThinkingEffort(effort)
	if effort == "off" {
		return false
	}
	if protocol == "anthropic" {
		body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": thinkingBudget(effort)}
		return true
	}
	body["reasoning_effort"] = effort
	return true
}

func removeThinking(body map[string]any, _ string) map[string]any {
	copied := make(map[string]any, len(body))
	for k, v := range body {
		if k == "reasoning_effort" || k == "thinking" {
			continue
		}
		copied[k] = v
	}
	return copied
}

func removeStreaming(body map[string]any) map[string]any {
	copied := make(map[string]any, len(body))
	for k, v := range body {
		if k == "stream_options" {
			continue
		}
		copied[k] = v
	}
	copied["stream"] = false
	return copied
}

func ensureOpenAIStreamUsage(req map[string]any) {
	options, _ := req["stream_options"].(map[string]any)
	if options == nil {
		options = map[string]any{}
	}
	options["include_usage"] = true
	req["stream_options"] = options
}

func thinkingBudget(effort string) int {
	switch effort {
	case "low":
		return 1024
	case "high":
		return 3072
	default:
		return 2048
	}
}

func retryWithoutThinking(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusUnprocessableEntity
}

func retryWithoutStreaming(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusUnprocessableEntity
}

func combineWarnings(values ...string) string {
	var warnings []string
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			warnings = append(warnings, text)
		}
	}
	return strings.Join(warnings, " ")
}

func firstOpenAIMessage(resp map[string]any) map[string]any {
	choices, _ := resp["choices"].([]any)
	if len(choices) == 0 {
		return map[string]any{}
	}
	choice, _ := choices[0].(map[string]any)
	message, _ := choice["message"].(map[string]any)
	if message == nil {
		return map[string]any{}
	}
	return message
}

func messageContent(message map[string]any) string {
	switch v := message["content"].(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			block, _ := item.(map[string]any)
			if text, _ := block["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func reasoningSummary(message map[string]any) string {
	for _, key := range []string{"reasoning_content", "reasoning", "thinking"} {
		if value, ok := message[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func anthropicThinking(resp map[string]any) string {
	var parts []string
	content, _ := resp["content"].([]any)
	for _, item := range content {
		block, _ := item.(map[string]any)
		if block == nil || stringArg(block, "type", "") != "thinking" {
			continue
		}
		if thinking := strings.TrimSpace(stringArg(block, "thinking", "")); thinking != "" {
			parts = append(parts, thinking)
		}
	}
	return strings.Join(parts, "\n")
}

type streamToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

type openAIStreamAccumulator struct {
	Content      strings.Builder
	Reasoning    strings.Builder
	ToolCalls    map[int]*streamToolCall
	ToolCallKeys []int
	FinishReason string
}

func parseOpenAIStream(body io.Reader, model string, emitDelta func(string) error) (map[string]any, convert.Usage, error) {
	acc := &openAIStreamAccumulator{ToolCalls: map[int]*streamToolCall{}}
	var usage convert.Usage
	err := readSSE(body, func(event sseEvent) error {
		if event.Data == "" || event.Data == "[DONE]" {
			return nil
		}
		chunk, err := decodeJSONEvent(event.Data)
		if err != nil {
			return err
		}
		if next := convert.UsageFromOpenAI(chunk); next.HasAny() {
			usage = mergeStreamUsage(usage, next)
		}
		return acc.AddChunk(chunk, emitDelta)
	})
	if err != nil {
		return nil, usage, err
	}
	return openAIResponseFromAccumulator(model, acc), usage, nil
}

func (a *openAIStreamAccumulator) AddChunk(chunk map[string]any, emitDelta func(string) error) error {
	for _, choiceValue := range asArray(chunk["choices"]) {
		choice, _ := choiceValue.(map[string]any)
		if reason := stringArg(choice, "finish_reason", ""); reason != "" {
			a.FinishReason = reason
		}
		delta, _ := choice["delta"].(map[string]any)
		if delta == nil {
			continue
		}
		if content := stringArg(delta, "content", ""); content != "" {
			a.Content.WriteString(content)
			if emitDelta != nil {
				if err := emitDelta(content); err != nil {
					return err
				}
			}
		}
		for _, key := range []string{"reasoning_content", "reasoning", "thinking"} {
			if reasoning := stringArg(delta, key, ""); reasoning != "" {
				a.Reasoning.WriteString(reasoning)
			}
		}
		for _, toolValue := range asArray(delta["tool_calls"]) {
			tool, _ := toolValue.(map[string]any)
			index := intArg(tool, "index", len(a.ToolCallKeys))
			call := a.toolCall(index)
			if id := stringArg(tool, "id", ""); id != "" {
				call.ID = id
			}
			fn, _ := tool["function"].(map[string]any)
			if name := stringArg(fn, "name", ""); name != "" {
				call.Name = name
			}
			if args := stringArg(fn, "arguments", ""); args != "" {
				call.Arguments.WriteString(args)
			}
		}
	}
	return nil
}

func (a *openAIStreamAccumulator) toolCall(index int) *streamToolCall {
	if call := a.ToolCalls[index]; call != nil {
		return call
	}
	call := &streamToolCall{}
	a.ToolCalls[index] = call
	a.ToolCallKeys = append(a.ToolCallKeys, index)
	return call
}

func openAIResponseFromAccumulator(model string, acc *openAIStreamAccumulator) map[string]any {
	message := map[string]any{
		"role":    "assistant",
		"content": acc.Content.String(),
	}
	if reasoning := strings.TrimSpace(acc.Reasoning.String()); reasoning != "" {
		message["reasoning_content"] = reasoning
	}
	if len(acc.ToolCallKeys) > 0 {
		calls := make([]any, 0, len(acc.ToolCallKeys))
		for _, index := range acc.ToolCallKeys {
			call := acc.ToolCalls[index]
			id := call.ID
			if id == "" {
				id = fmt.Sprintf("call_%d", index)
			}
			args := call.Arguments.String()
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			calls = append(calls, map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": args,
				},
			})
		}
		message["tool_calls"] = calls
	}
	finishReason := any(acc.FinishReason)
	if finishReason == "" {
		finishReason = nil
	}
	return map[string]any{
		"model": model,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		}},
	}
}

type anthropicStreamBlock struct {
	Type      string
	ID        string
	Name      string
	Arguments strings.Builder
}

type anthropicStreamAccumulator struct {
	Content      strings.Builder
	Reasoning    strings.Builder
	Blocks       map[int]*anthropicStreamBlock
	BlockKeys    []int
	FinishReason string
}

func parseAnthropicStream(body io.Reader, model string, emitDelta func(string) error) (map[string]any, convert.Usage, error) {
	acc := &anthropicStreamAccumulator{Blocks: map[int]*anthropicStreamBlock{}}
	var usage convert.Usage
	err := readSSE(body, func(event sseEvent) error {
		if event.Data == "" {
			return nil
		}
		payload, err := decodeJSONEvent(event.Data)
		if err != nil {
			return err
		}
		eventName := event.Event
		if eventName == "" {
			eventName = stringArg(payload, "type", "")
		}
		switch eventName {
		case "message_start":
			if msg, ok := payload["message"].(map[string]any); ok {
				usage = mergeStreamUsage(usage, convert.UsageFromAnthropic(msg))
			}
		case "content_block_start":
			acc.StartBlock(payload, emitDelta)
		case "content_block_delta":
			if err := acc.AddDelta(payload, emitDelta); err != nil {
				return err
			}
		case "message_delta":
			usage = mergeStreamUsage(usage, convert.UsageFromAnthropic(payload))
			delta, _ := payload["delta"].(map[string]any)
			if reason := stringArg(delta, "stop_reason", ""); reason != "" {
				acc.FinishReason = anthropicStopToOpenAI(reason)
			}
		case "error":
			if errObj, ok := payload["error"].(map[string]any); ok {
				return fmt.Errorf("upstream error: %s", stringArg(errObj, "message", "anthropic stream error"))
			}
			return fmt.Errorf("upstream error: anthropic stream error")
		}
		return nil
	})
	if err != nil {
		return nil, usage, err
	}
	return anthropicOpenAIResponse(model, acc), usage, nil
}

func (a *anthropicStreamAccumulator) StartBlock(payload map[string]any, emitDelta func(string) error) {
	index := intArg(payload, "index", len(a.BlockKeys))
	blockPayload, _ := payload["content_block"].(map[string]any)
	block := a.block(index)
	block.Type = stringArg(blockPayload, "type", "")
	switch block.Type {
	case "text":
		if text := stringArg(blockPayload, "text", ""); text != "" {
			a.Content.WriteString(text)
			if emitDelta != nil {
				_ = emitDelta(text)
			}
		}
	case "thinking":
		if thinking := stringArg(blockPayload, "thinking", ""); thinking != "" {
			a.Reasoning.WriteString(thinking)
		}
	case "tool_use":
		block.ID = stringArg(blockPayload, "id", "")
		block.Name = stringArg(blockPayload, "name", "")
		if input, ok := blockPayload["input"].(map[string]any); ok && len(input) > 0 {
			raw, _ := json.Marshal(input)
			block.Arguments.Write(raw)
		}
	}
}

func (a *anthropicStreamAccumulator) AddDelta(payload map[string]any, emitDelta func(string) error) error {
	index := intArg(payload, "index", len(a.BlockKeys))
	block := a.block(index)
	delta, _ := payload["delta"].(map[string]any)
	switch stringArg(delta, "type", "") {
	case "text_delta":
		text := stringArg(delta, "text", "")
		a.Content.WriteString(text)
		if emitDelta != nil {
			return emitDelta(text)
		}
	case "thinking_delta":
		a.Reasoning.WriteString(stringArg(delta, "thinking", ""))
	case "input_json_delta":
		block.Arguments.WriteString(stringArg(delta, "partial_json", ""))
	}
	return nil
}

func (a *anthropicStreamAccumulator) block(index int) *anthropicStreamBlock {
	if block := a.Blocks[index]; block != nil {
		return block
	}
	block := &anthropicStreamBlock{}
	a.Blocks[index] = block
	a.BlockKeys = append(a.BlockKeys, index)
	return block
}

func anthropicOpenAIResponse(model string, acc *anthropicStreamAccumulator) map[string]any {
	openAI := &openAIStreamAccumulator{
		ToolCalls:    map[int]*streamToolCall{},
		ToolCallKeys: []int{},
		FinishReason: acc.FinishReason,
	}
	openAI.Content.WriteString(acc.Content.String())
	openAI.Reasoning.WriteString(acc.Reasoning.String())
	for _, index := range acc.BlockKeys {
		block := acc.Blocks[index]
		if block.Type != "tool_use" {
			continue
		}
		call := openAI.toolCall(index)
		call.ID = block.ID
		call.Name = block.Name
		call.Arguments.WriteString(block.Arguments.String())
	}
	return openAIResponseFromAccumulator(model, openAI)
}

func extractToolCalls(message map[string]any) []toolCall {
	raw, _ := message["tool_calls"].([]any)
	calls := make([]toolCall, 0, len(raw))
	for i, item := range raw {
		tc, _ := item.(map[string]any)
		fn, _ := tc["function"].(map[string]any)
		id, _ := tc["id"].(string)
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		name, _ := fn["name"].(string)
		args, _ := fn["arguments"].(string)
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		calls = append(calls, toolCall{
			ID:        id,
			Name:      name,
			Arguments: args,
			OpenAI: map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      name,
					"arguments": args,
				},
			},
		})
	}
	return calls
}

func normalizedToolCalls(calls []toolCall) []any {
	out := make([]any, 0, len(calls))
	for _, call := range calls {
		out = append(out, call.OpenAI)
	}
	return out
}

func shouldSummarizeConversationTitle(conv store.ChatConversation, history []store.ChatMessage) bool {
	if !conv.TitleAutoGenerated {
		return false
	}
	for _, msg := range history {
		if msg.Role == store.ChatRoleAssistant {
			return false
		}
	}
	return true
}

func titleTranscript(history []store.ChatMessage) string {
	type titleMessage struct {
		role    string
		content string
	}
	messages := make([]titleMessage, 0, len(history))
	for _, msg := range history {
		if msg.Role != store.ChatRoleUser && msg.Role != store.ChatRoleAssistant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		messages = append(messages, titleMessage{role: msg.Role, content: content})
	}
	if len(messages) == 0 {
		return ""
	}

	selected := make(map[int]bool)
	for i, msg := range messages {
		if msg.role == store.ChatRoleUser {
			selected[i] = true
			break
		}
	}
	for i, msg := range messages {
		if msg.role == store.ChatRoleAssistant {
			selected[i] = true
			break
		}
	}
	recentStart := len(messages) - 6
	if recentStart < 0 {
		recentStart = 0
	}
	for i := recentStart; i < len(messages); i++ {
		selected[i] = true
	}

	var builder strings.Builder
	for i, msg := range messages {
		if !selected[i] {
			continue
		}
		role := "User"
		if msg.role == store.ChatRoleAssistant {
			role = "Assistant"
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(role)
		builder.WriteString(":\n")
		builder.WriteString(truncateForTitle(msg.content))
	}
	return builder.String()
}

func fallbackTitleFromMessages(history []store.ChatMessage) string {
	for _, msg := range history {
		if msg.Role == store.ChatRoleUser {
			if title := titleFromMessage(msg.Content); !isDefaultGeneratedTitle(title) {
				return title
			}
		}
	}
	for _, msg := range history {
		if msg.Role == store.ChatRoleAssistant {
			if title := titleFromMessage(msg.Content); !isDefaultGeneratedTitle(title) {
				return title
			}
		}
	}
	return ""
}

func isDefaultGeneratedTitle(title string) bool {
	switch strings.TrimSpace(title) {
	case "", "New chat", "新对话":
		return true
	default:
		return false
	}
}

func toolLimitMessage(maxToolCalls int) string {
	if maxToolCalls <= 0 {
		return "Tool use is disabled by the current maximum tool call setting."
	}
	return fmt.Sprintf("The maximum tool call limit (%d) was reached before the model produced a final answer.", maxToolCalls)
}

func truncateForTitle(content string) string {
	content = strings.Join(strings.Fields(content), " ")
	runes := []rune(content)
	if len(runes) > 1200 {
		return string(runes[:1200])
	}
	return content
}

func cleanGeneratedTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`“”‘’")
	title = strings.TrimSpace(title)
	for _, prefix := range []string{"- ", "* ", "• "} {
		title = strings.TrimPrefix(title, prefix)
	}
	if idx := strings.IndexAny(title, "\r\n"); idx >= 0 {
		title = title[:idx]
	}
	title = strings.TrimSpace(strings.Trim(title, "\"'`“”‘’"))
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60])
	}
	return title
}

func titleFromMessage(content string) string {
	content = strings.Join(strings.Fields(content), " ")
	runes := []rune(content)
	if len(runes) > 60 {
		return string(runes[:60])
	}
	if content == "" {
		return "New chat"
	}
	return content
}

func emitJSON(emit Emitter, event string, payload any) error {
	if emit == nil {
		return nil
	}
	return emit(event, payload)
}

func mergeUsage(a, b convert.Usage) convert.Usage {
	return convert.Usage{
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
	}
}

func mergeStreamUsage(current, next convert.Usage) convert.Usage {
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

func decodeJSONEvent(data string) (map[string]any, error) {
	var out map[string]any
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	err := dec.Decode(&out)
	return out, err
}

func asArray(value any) []any {
	if arr, ok := value.([]any); ok {
		return arr
	}
	return nil
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

func upstreamURL(provider store.Provider) string {
	base := strings.TrimRight(provider.BaseAPI, "/")
	if provider.Protocol == "anthropic" {
		return base + "/messages"
	}
	return base + "/chat/completions"
}

func upstreamErrorMessage(raw []byte) string {
	message := strings.TrimSpace(string(raw))
	var parsed map[string]any
	if json.Unmarshal(raw, &parsed) == nil {
		if errObj, ok := parsed["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok {
				message = msg
			}
		}
	}
	if message == "" {
		return "upstream error"
	}
	return message
}

func stringArg(args map[string]any, key, fallback string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return fallback
}

func intArg(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		if n, err := value.Int64(); err == nil {
			return int(n)
		}
	}
	return fallback
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	if value, ok := args[key].(bool); ok {
		return value
	}
	return fallback
}

func waitUntilArg(args map[string]any, key string) string {
	switch strings.ToLower(strings.TrimSpace(stringArg(args, key, "load"))) {
	case "load", "domcontentloaded", "networkidle":
		return strings.ToLower(strings.TrimSpace(stringArg(args, key, "load")))
	default:
		return "load"
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func toolError(message string) string {
	raw, _ := json.Marshal(map[string]any{"error": message})
	return string(raw)
}

func truncateToolResult(value string) string {
	if len([]rune(value)) <= maxToolResultChars {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxToolResultChars]) + "\n\n[truncated]"
}
