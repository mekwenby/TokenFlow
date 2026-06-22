package convert

import "testing"

func TestAnthropicRequestToOpenAIMultimodalAndTools(t *testing.T) {
	req := JSON{
		"system":     "system prompt",
		"model":      "client-model",
		"max_tokens": 32,
		"messages": []any{JSON{
			"role": "user",
			"content": []any{
				JSON{"type": "text", "text": "look"},
				JSON{"type": "image", "source": JSON{"type": "url", "url": "https://example.com/a.png"}},
			},
		}},
		"tools": []any{JSON{"name": "lookup", "description": "Search", "input_schema": JSON{"type": "object"}}},
		"tool_choice": JSON{
			"type": "tool",
			"name": "lookup",
		},
	}

	out := AnthropicRequestToOpenAI(req, "upstream-model")
	if out["model"] != "upstream-model" {
		t.Fatalf("model was not overridden: %#v", out["model"])
	}
	messages := out["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected system + user messages, got %d", len(messages))
	}
	user := messages[1].(JSON)
	parts := user["content"].([]any)
	if len(parts) != 2 {
		t.Fatalf("expected text and image in the same message, got %#v", parts)
	}
	if parts[0].(JSON)["type"] != "text" || parts[1].(JSON)["type"] != "image_url" {
		t.Fatalf("unexpected content parts: %#v", parts)
	}
	if len(out["tools"].([]any)) != 1 {
		t.Fatalf("expected tool mapping")
	}
	choice := out["tool_choice"].(JSON)
	if choice["type"] != "function" {
		t.Fatalf("unexpected tool choice: %#v", choice)
	}
}

func TestAnthropicRequestToOpenAIThinkingAndToolUse(t *testing.T) {
	req := JSON{
		"model":      "client-model",
		"max_tokens": 32,
		"messages": []any{JSON{
			"role": "assistant",
			"content": []any{
				JSON{"type": "thinking", "thinking": "I should call the lookup tool."},
				JSON{"type": "tool_use", "id": "call_123", "name": "lookup", "input": JSON{"q": "x"}},
			},
		}},
	}

	out := AnthropicRequestToOpenAI(req, "upstream-model")
	messages := out["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected one assistant message, got %d", len(messages))
	}
	assistant := messages[0].(JSON)
	if assistant["reasoning_content"] != "I should call the lookup tool." {
		t.Fatalf("reasoning content was not preserved: %#v", assistant)
	}
	if len(assistant["tool_calls"].([]any)) != 1 {
		t.Fatalf("tool call was not preserved: %#v", assistant)
	}
}

func TestAnthropicRequestToOpenAIEmptyThinkingPassback(t *testing.T) {
	req := JSON{
		"model":      "client-model",
		"max_tokens": 32,
		"messages": []any{JSON{
			"role": "assistant",
			"content": []any{
				JSON{"type": "thinking", "thinking": ""},
				JSON{"type": "tool_use", "id": "call_123", "name": "lookup", "input": JSON{}},
			},
		}},
	}

	out := AnthropicRequestToOpenAI(req, "upstream-model")
	assistant := out["messages"].([]any)[0].(JSON)
	reasoning, ok := assistant["reasoning_content"]
	if !ok || reasoning != "" {
		t.Fatalf("empty reasoning content was not passed back: %#v", assistant)
	}
}

func TestOpenAIResponseToAnthropicReasoningContent(t *testing.T) {
	resp := JSON{
		"choices": []any{JSON{
			"finish_reason": "stop",
			"message": JSON{
				"reasoning_content": "I should answer directly.",
				"content":           "hello",
			},
		}},
		"usage": JSON{"prompt_tokens": 10, "completion_tokens": 4},
	}

	out := OpenAIResponseToAnthropic(resp, "model")
	content := out["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected thinking + text blocks, got %#v", content)
	}
	thinking := content[0].(JSON)
	if thinking["type"] != "thinking" || thinking["thinking"] != "I should answer directly." {
		t.Fatalf("unexpected thinking block: %#v", thinking)
	}
	text := content[1].(JSON)
	if text["type"] != "text" || text["text"] != "hello" {
		t.Fatalf("unexpected text block: %#v", text)
	}
}

func TestOpenAIResponseToAnthropicToolCall(t *testing.T) {
	resp := JSON{
		"choices": []any{JSON{
			"finish_reason": "tool_calls",
			"message": JSON{
				"content": nil,
				"tool_calls": []any{JSON{
					"id":   "call_123",
					"type": "function",
					"function": JSON{
						"name":      "lookup",
						"arguments": `{"q":"x"}`,
					},
				}},
			},
		}},
		"usage": JSON{"prompt_tokens": 10, "completion_tokens": 4},
	}
	out := OpenAIResponseToAnthropic(resp, "model")
	if out["stop_reason"] != "tool_use" {
		t.Fatalf("unexpected stop reason: %#v", out["stop_reason"])
	}
	content := out["content"].([]any)
	block := content[0].(JSON)
	if block["type"] != "tool_use" || block["id"] != "call_123" || block["name"] != "lookup" {
		t.Fatalf("unexpected tool block: %#v", block)
	}
	input := block["input"].(map[string]any)
	if input["q"] != "x" {
		t.Fatalf("tool arguments were not parsed: %#v", input)
	}
}

func TestUsageIncludesCacheTokens(t *testing.T) {
	openAI := JSON{
		"usage": JSON{
			"prompt_tokens":     20,
			"completion_tokens": 5,
			"prompt_tokens_details": JSON{
				"cached_tokens": 8,
			},
		},
	}
	usage := UsageFromOpenAI(openAI)
	if usage.InputTokens != 20 || usage.OutputTokens != 5 || usage.CacheReadTokens != 8 {
		t.Fatalf("unexpected OpenAI usage: %#v", usage)
	}
	if rate := CacheHitRate(usage.InputTokens, usage.CacheReadTokens); rate != 0.4 {
		t.Fatalf("unexpected cache hit rate: %v", rate)
	}

	anthropic := JSON{
		"usage": JSON{
			"input_tokens":                30,
			"output_tokens":               7,
			"cache_read_input_tokens":     12,
			"cache_creation_input_tokens": 4,
		},
	}
	usage = UsageFromAnthropic(anthropic)
	if usage.InputTokens != 30 || usage.OutputTokens != 7 || usage.CacheReadTokens != 12 || usage.CacheCreationTokens != 4 {
		t.Fatalf("unexpected Anthropic usage: %#v", usage)
	}
}
