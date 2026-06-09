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
