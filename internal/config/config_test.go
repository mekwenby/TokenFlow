package config

import "testing"

func TestLoadChatContextMaxRunes(t *testing.T) {
	t.Setenv("GATEWAY_DATA_DIR", t.TempDir())
	t.Setenv("CHAT_CONTEXT_MAX_RUNES", "300000")
	if got := Load().ChatContextMaxRunes; got != 300000 {
		t.Fatalf("unexpected configured chat context limit: %d", got)
	}
	t.Setenv("CHAT_CONTEXT_MAX_RUNES", "invalid")
	if got := Load().ChatContextMaxRunes; got != 262144 {
		t.Fatalf("invalid chat context limit should use default: %d", got)
	}
}
