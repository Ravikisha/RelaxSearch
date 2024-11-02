package config

import (
	"os"
	"strconv"
)

type Config struct {
	ElasticsearchURL string
	DepthLimit       int
}

// LoadConfig loads configuration from environment variables or defaults
func LoadConfig() Config {
	return Config{
		ElasticsearchURL: getEnv("ELASTICSEARCH_URL", "http://127.0.0.1:7000"),
		DepthLimit:       getEnvAsInt("DEPTH_LIMIT", 2),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultValue int) int {
	if valueStr := os.Getenv(name); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
