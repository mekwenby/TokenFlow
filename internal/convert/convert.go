package convert

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type JSON = map[string]any

type Usage struct {
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
}

func (u Usage) HasAny() bool {
	return u.InputTokens > 0 || u.OutputTokens > 0 || u.CacheReadTokens > 0 || u.CacheCreationTokens > 0
}

func CacheHitRate(inputTokens, cacheReadTokens int64) float64 {
	if inputTokens <= 0 || cacheReadTokens <= 0 {
		return 0
	}
	return float64(cacheReadTokens) / float64(inputTokens)
}

func IsStream(req JSON) bool {
	v, _ := req["stream"].(bool)
	return v
}

func Model(req JSON) string {
	model, _ := req["model"].(string)
	return model
}

func AnthropicRequestToOpenAI(req JSON, model string) JSON {
	messages := make([]any, 0)
	if system := extractText(req["system"]); system != "" {
		messages = append(messages, JSON{"role": "system", "content": system})
	}

	for _, item := range asArray(req["messages"]) {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := stringValue(msg["role"])
		content := msg["content"]
		switch v := content.(type) {
		case string:
			messages = append(messages, JSON{"role": role, "content": v})
		case []any:
			openAIContent := make([]any, 0)
			textParts := make([]string, 0)
			toolCalls := make([]any, 0)
			reasoningParts := make([]string, 0)
			hasReasoning := false
			for _, blockValue := range v {
				block, ok := blockValue.(map[string]any)
				if !ok {
					continue
				}
				switch stringValue(block["type"]) {
				case "thinking":
					if role == "assistant" {
						if thinking, ok := block["thinking"].(string); ok {
							hasReasoning = true
							reasoningParts = append(reasoningParts, thinking)
						}
					}
				case "text":
					text := stringValue(block["text"])
					textParts = append(textParts, text)
					openAIContent = append(openAIContent, JSON{"type": "text", "text": text})
				case "image":
					if image := anthropicImageToOpenAI(block); image != nil {
						openAIContent = append(openAIContent, image)
					}
				case "tool_use":
					toolCalls = append(toolCalls, JSON{
						"id":   stringValue(block["id"]),
						"type": "function",
						"function": JSON{
							"name":      stringValue(block["name"]),
							"arguments": marshalObject(block["input"]),
						},
					})
				case "tool_result":
					result := JSON{
						"role":         "tool",
						"tool_call_id": stringValue(block["tool_use_id"]),
						"content":      extractText(block["content"]),
					}
					messages = append(messages, result)
				}
			}
			if len(openAIContent) > 0 || len(toolCalls) > 0 || hasReasoning {
				out := JSON{"role": role}
				if len(openAIContent) == 0 {
					out["content"] = strings.Join(textParts, "\n")
				} else if onlyText(openAIContent) && len(toolCalls) == 0 {
					out["content"] = strings.Join(textParts, "\n")
				} else {
					out["content"] = openAIContent
				}
				if len(toolCalls) > 0 {
					out["tool_calls"] = toolCalls
					if _, ok := out["content"]; !ok {
						out["content"] = nil
					}
				}
				if hasReasoning {
					out["reasoning_content"] = strings.Join(reasoningParts, "\n")
				}
				messages = append(messages, out)
			}
		}
	}

	out := JSON{
		"model":      model,
		"messages":   messages,
		"max_tokens": intValue(req["max_tokens"], 4096),
	}
	copyIfPresent(out, req, "temperature")
	copyIfPresent(out, req, "top_p")
	copyIfPresent(out, req, "stream")
	if stops := asArray(req["stop_sequences"]); len(stops) > 0 {
		out["stop"] = stops
	}
	if tools := anthropicToolsToOpenAI(req["tools"]); len(tools) > 0 {
		out["tools"] = tools
	}
	if choice, ok := req["tool_choice"].(map[string]any); ok {
		out["tool_choice"] = anthropicToolChoiceToOpenAI(choice)
	}
	return out
}

func OpenAIRequestToAnthropic(req JSON, model string) JSON {
	systemParts := make([]string, 0)
	messages := make([]any, 0)
	for _, item := range asArray(req["messages"]) {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := stringValue(msg["role"])
		if role == "system" || role == "developer" {
			text := extractOpenAIMessageText(msg["content"])
			if text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		}
		switch role {
		case "tool":
			messages = append(messages, JSON{
				"role": "user",
				"content": []any{JSON{
					"type":        "tool_result",
					"tool_use_id": stringValue(msg["tool_call_id"]),
					"content":     extractOpenAIMessageText(msg["content"]),
				}},
			})
		case "assistant":
			blocks := openAIContentToAnthropicBlocks(msg["content"])
			for _, tc := range asArray(msg["tool_calls"]) {
				if block := openAIToolCallToAnthropic(tc); block != nil {
					blocks = append(blocks, block)
				}
			}
			if len(blocks) == 0 {
				blocks = []any{JSON{"type": "text", "text": ""}}
			}
			messages = append(messages, JSON{"role": "assistant", "content": blocks})
		default:
			blocks := openAIContentToAnthropicBlocks(msg["content"])
			if len(blocks) == 1 {
				if block, ok := blocks[0].(JSON); ok && block["type"] == "text" {
					messages = append(messages, JSON{"role": "user", "content": block["text"]})
					continue
				}
			}
			messages = append(messages, JSON{"role": "user", "content": blocks})
		}
	}

	out := JSON{
		"model":      model,
		"messages":   messages,
		"max_tokens": intValue(firstPresent(req, "max_tokens", "max_completion_tokens"), 4096),
	}
	if len(systemParts) > 0 {
		out["system"] = strings.Join(systemParts, "\n")
	}
	copyIfPresent(out, req, "temperature")
	copyIfPresent(out, req, "top_p")
	copyIfPresent(out, req, "stream")
	if stop, ok := req["stop"]; ok {
		out["stop_sequences"] = stop
	}
	if tools := openAIToolsToAnthropic(req["tools"]); len(tools) > 0 {
		out["tools"] = tools
	}
	if choice, ok := req["tool_choice"]; ok {
		out["tool_choice"] = openAIToolChoiceToAnthropic(choice)
	}
	return out
}

func OpenAIResponseToAnthropic(resp JSON, model string) JSON {
	choices := asArray(resp["choices"])
	var choice map[string]any
	if len(choices) > 0 {
		choice, _ = choices[0].(map[string]any)
	}
	message, _ := choice["message"].(map[string]any)
	blocks := make([]any, 0)
	if reasoning, ok := message["reasoning_content"].(string); ok {
		blocks = append(blocks, JSON{"type": "thinking", "thinking": reasoning})
	}
	if text := stringValue(message["content"]); text != "" {
		blocks = append(blocks, JSON{"type": "text", "text": text})
	}
	for _, tc := range asArray(message["tool_calls"]) {
		if block := openAIToolCallToAnthropic(tc); block != nil {
			blocks = append(blocks, block)
		}
	}
	for i, block := range blocks {
		if m, ok := block.(JSON); ok {
			m["index"] = i
		}
	}
	usage, _ := resp["usage"].(map[string]any)
	parsedUsage := UsageFromOpenAI(resp)
	return JSON{
		"id":            "msg_" + randomHex(12),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       blocks,
		"stop_reason":   openAIFinishToAnthropic(stringValue(choice["finish_reason"])),
		"stop_sequence": nil,
		"usage": JSON{
			"input_tokens":                intValue(usage["prompt_tokens"], 0),
			"output_tokens":               intValue(usage["completion_tokens"], 0),
			"cache_read_input_tokens":     parsedUsage.CacheReadTokens,
			"cache_creation_input_tokens": parsedUsage.CacheCreationTokens,
		},
	}
}

func AnthropicResponseToOpenAI(resp JSON, model string) JSON {
	content := make([]string, 0)
	toolCalls := make([]any, 0)
	for _, item := range asArray(resp["content"]) {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(block["type"]) {
		case "text":
			content = append(content, stringValue(block["text"]))
		case "tool_use":
			toolCalls = append(toolCalls, JSON{
				"id":   stringValue(block["id"]),
				"type": "function",
				"function": JSON{
					"name":      stringValue(block["name"]),
					"arguments": marshalObject(block["input"]),
				},
			})
		}
	}
	message := JSON{"role": "assistant", "content": strings.Join(content, "")}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	usage, _ := resp["usage"].(map[string]any)
	parsedUsage := UsageFromAnthropic(resp)
	out := JSON{
		"id":      "chatcmpl_" + randomHex(12),
		"object":  "chat.completion",
		"created": unixNow(),
		"model":   model,
		"choices": []any{JSON{
			"index":         0,
			"message":       message,
			"finish_reason": anthropicStopToOpenAI(stringValue(resp["stop_reason"])),
		}},
		"usage": JSON{
			"prompt_tokens":     intValue(usage["input_tokens"], 0),
			"completion_tokens": intValue(usage["output_tokens"], 0),
			"total_tokens":      intValue(usage["input_tokens"], 0) + intValue(usage["output_tokens"], 0),
		},
	}
	if parsedUsage.CacheReadTokens > 0 {
		out["usage"].(JSON)["prompt_tokens_details"] = JSON{"cached_tokens": parsedUsage.CacheReadTokens}
	}
	return out
}

func UsageFromOpenAI(resp JSON) Usage {
	usage, _ := resp["usage"].(map[string]any)
	if usage == nil {
		return Usage{}
	}
	return Usage{
		InputTokens:         int64(intValue(usage["prompt_tokens"], 0)),
		OutputTokens:        int64(intValue(usage["completion_tokens"], 0)),
		CacheReadTokens:     int64(openAICacheReadTokens(usage)),
		CacheCreationTokens: int64(intValue(usage["cache_creation_input_tokens"], 0)),
	}
}

func UsageFromAnthropic(resp JSON) Usage {
	usage, _ := resp["usage"].(map[string]any)
	if usage == nil {
		return Usage{}
	}
	return Usage{
		InputTokens:         int64(intValue(usage["input_tokens"], 0)),
		OutputTokens:        int64(intValue(usage["output_tokens"], 0)),
		CacheReadTokens:     int64(intValue(usage["cache_read_input_tokens"], 0)),
		CacheCreationTokens: int64(intValue(usage["cache_creation_input_tokens"], 0)),
	}
}

func BuildOpenAIChunk(model string, delta JSON, finishReason any) JSON {
	choice := JSON{"index": 0, "delta": delta, "finish_reason": finishReason}
	return JSON{
		"id":      "chatcmpl_" + randomHex(12),
		"object":  "chat.completion.chunk",
		"created": unixNow(),
		"model":   model,
		"choices": []any{choice},
	}
}

func BuildAnthropicEvent(eventType string, data JSON) []byte {
	if data["type"] == nil {
		data["type"] = eventType
	}
	body, _ := json.Marshal(data)
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, body))
}

func MarshalSSEData(v any) []byte {
	body, _ := json.Marshal(v)
	return []byte(fmt.Sprintf("data: %s\n\n", body))
}

func anthropicImageToOpenAI(block map[string]any) any {
	source, _ := block["source"].(map[string]any)
	switch stringValue(source["type"]) {
	case "base64":
		media := stringValue(source["media_type"])
		if media == "" {
			media = "image/png"
		}
		return JSON{"type": "image_url", "image_url": JSON{"url": "data:" + media + ";base64," + stringValue(source["data"])}}
	case "url":
		return JSON{"type": "image_url", "image_url": JSON{"url": stringValue(source["url"])}}
	default:
		return nil
	}
}

func anthropicToolsToOpenAI(value any) []any {
	tools := make([]any, 0)
	for _, item := range asArray(value) {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tools = append(tools, JSON{
			"type": "function",
			"function": JSON{
				"name":        stringValue(tool["name"]),
				"description": stringValue(tool["description"]),
				"parameters":  fallbackObject(tool["input_schema"]),
			},
		})
	}
	return tools
}

func openAIToolsToAnthropic(value any) []any {
	tools := make([]any, 0)
	for _, item := range asArray(value) {
		tool, ok := item.(map[string]any)
		if !ok || stringValue(tool["type"]) != "function" {
			continue
		}
		fn, _ := tool["function"].(map[string]any)
		tools = append(tools, JSON{
			"name":         stringValue(fn["name"]),
			"description":  stringValue(fn["description"]),
			"input_schema": fallbackObject(fn["parameters"]),
		})
	}
	return tools
}

func anthropicToolChoiceToOpenAI(choice map[string]any) any {
	switch stringValue(choice["type"]) {
	case "any":
		return "required"
	case "tool":
		return JSON{"type": "function", "function": JSON{"name": stringValue(choice["name"])}}
	case "none":
		return "none"
	default:
		return "auto"
	}
}

func openAIToolChoiceToAnthropic(choice any) any {
	if s, ok := choice.(string); ok {
		switch s {
		case "required":
			return JSON{"type": "any"}
		case "none":
			return JSON{"type": "none"}
		default:
			return JSON{"type": "auto"}
		}
	}
	m, _ := choice.(map[string]any)
	fn, _ := m["function"].(map[string]any)
	if stringValue(m["type"]) == "function" && stringValue(fn["name"]) != "" {
		return JSON{"type": "tool", "name": stringValue(fn["name"])}
	}
	return JSON{"type": "auto"}
}

func openAIContentToAnthropicBlocks(content any) []any {
	if text, ok := content.(string); ok {
		return []any{JSON{"type": "text", "text": text}}
	}
	blocks := make([]any, 0)
	for _, partValue := range asArray(content) {
		part, ok := partValue.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(part["type"]) {
		case "text":
			blocks = append(blocks, JSON{"type": "text", "text": stringValue(part["text"])})
		case "image_url":
			imageURL, _ := part["image_url"].(map[string]any)
			url := stringValue(imageURL["url"])
			source := JSON{"type": "url", "url": url}
			if strings.HasPrefix(url, "data:") {
				media := "image/png"
				data := url
				if comma := strings.Index(url, ","); comma >= 0 {
					head := url[:comma]
					data = url[comma+1:]
					if semi := strings.Index(head, ";"); strings.HasPrefix(head, "data:") && semi > 5 {
						media = head[5:semi]
					}
				}
				source = JSON{"type": "base64", "media_type": media, "data": data}
			}
			blocks = append(blocks, JSON{"type": "image", "source": source})
		}
	}
	return blocks
}

func openAIToolCallToAnthropic(value any) any {
	tc, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	fn, _ := tc["function"].(map[string]any)
	var input any = JSON{}
	if args := stringValue(fn["arguments"]); args != "" {
		var parsed any
		if json.Unmarshal([]byte(args), &parsed) == nil {
			input = parsed
		}
	}
	return JSON{
		"type":  "tool_use",
		"id":    stringValue(tc["id"]),
		"name":  stringValue(fn["name"]),
		"input": input,
	}
}

func extractText(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0)
		for _, item := range v {
			switch block := item.(type) {
			case string:
				parts = append(parts, block)
			case map[string]any:
				if stringValue(block["type"]) == "text" {
					parts = append(parts, stringValue(block["text"]))
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(v)
	}
}

func extractOpenAIMessageText(content any) string {
	if text, ok := content.(string); ok {
		return text
	}
	parts := make([]string, 0)
	for _, item := range asArray(content) {
		part, ok := item.(map[string]any)
		if ok && stringValue(part["type"]) == "text" {
			parts = append(parts, stringValue(part["text"]))
		}
	}
	return strings.Join(parts, "\n")
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

func openAICacheReadTokens(usage map[string]any) int {
	if cached := intValue(usage["cache_read_input_tokens"], 0); cached > 0 {
		return cached
	}
	details, _ := usage["prompt_tokens_details"].(map[string]any)
	return intValue(details["cached_tokens"], 0)
}

func onlyText(parts []any) bool {
	for _, partValue := range parts {
		part, ok := partValue.(map[string]any)
		if !ok || stringValue(part["type"]) != "text" {
			return false
		}
	}
	return true
}

func copyIfPresent(dst, src JSON, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}

func firstPresent(src JSON, keys ...string) any {
	for _, key := range keys {
		if value, ok := src[key]; ok {
			return value
		}
	}
	return nil
}

func asArray(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	default:
		return nil
	}
}

func fallbackObject(value any) any {
	if value == nil {
		return JSON{"type": "object", "properties": JSON{}}
	}
	return value
}

func marshalObject(value any) string {
	if value == nil {
		return "{}"
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func intValue(value any, fallback int) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
	}
	return fallback
}

func randomHex(bytes int) string {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "000000000000000000000000"
	}
	return hex.EncodeToString(raw)
}

func unixNow() int64 {
	return time.Now().Unix()
}
