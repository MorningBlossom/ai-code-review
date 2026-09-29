package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	GitHubWebhookSecret  string
	GitHubAppID          int64
	GitHubPrivateKeyPath string
	ModelBaseURL         string
	ModelName            string
	ModelTimeoutSeconds  int
	AppEnv               string
}

func Load() Config {

	_ = godotenv.Load()

	return Config{
		GitHubWebhookSecret:  os.Getenv("GITHUB_WEBHOOK_SECRET"),
		ModelBaseURL:         os.Getenv("MODEL_BASE_URL"),
		ModelName:            os.Getenv("MODEL_NAME"),
		GitHubAppID:          getInt64Env("GITHUB_APP_ID", 0),
		GitHubPrivateKeyPath: os.Getenv("GITHUB_PRIVATE_KEY_PATH"),
		AppEnv:               os.Getenv("APP_ENV"),
		ModelTimeoutSeconds: getIntEnv(
			"MODEL_TIMEOUT_SECONDS",
			120,
		),
	}
}

func getIntEnv(
	key string,
	defaultValue int,
) int {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

func getInt64Env(key string, defaultValue int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}

	return parsed
}
