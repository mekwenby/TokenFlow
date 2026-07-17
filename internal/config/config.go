package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Addr                string
	DataDir             string
	DBPath              string
	SecretPath          string
	InfoFlowBaseURL     string
	ChatContextMaxRunes int
}

func Load() Config {
	dataDir := getenv("GATEWAY_DATA_DIR", "data")
	addr := getenv("GATEWAY_ADDR", ":8019")
	return Config{
		Addr:                addr,
		DataDir:             dataDir,
		DBPath:              filepath.Join(dataDir, "gateway.db"),
		SecretPath:          filepath.Join(dataDir, "app.secret"),
		InfoFlowBaseURL:     getenv("INFOFLOW_BASE_URL", "https://infoflow.030399.xyz"),
		ChatContextMaxRunes: getenvInt("CHAT_CONTEXT_MAX_RUNES", 262144),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
