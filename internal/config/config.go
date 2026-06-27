package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr            string
	DataDir         string
	DBPath          string
	SecretPath      string
	InfoFlowBaseURL string
}

func Load() Config {
	dataDir := getenv("GATEWAY_DATA_DIR", "data")
	addr := getenv("GATEWAY_ADDR", ":8019")
	return Config{
		Addr:            addr,
		DataDir:         dataDir,
		DBPath:          filepath.Join(dataDir, "gateway.db"),
		SecretPath:      filepath.Join(dataDir, "app.secret"),
		InfoFlowBaseURL: getenv("INFOFLOW_BASE_URL", "https://infoflow.030399.xyz"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
