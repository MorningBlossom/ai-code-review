package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	GitHubWebhookSecret  string
	GitHubAppID          int64
	GitHubPrivateKeyPath string
	GitHubOrganization   string
	ModelBaseURL         string
	ModelName            string
	ModelTimeoutSeconds  int
	AppEnv               string
	APIToken             string
}

func Load() Config {

	_ = godotenv.Load()

	return Config{
		GitHubWebhookSecret:  os.Getenv("GITHUB_WEBHOOK_SECRET"),
		ModelBaseURL:         os.Getenv("MODEL_BASE_URL"),
		ModelName:            os.Getenv("MODEL_NAME"),
		GitHubAppID:          getInt64Env("GITHUB_APP_ID", 0),
		GitHubPrivateKeyPath: os.Getenv("GITHUB_PRIVATE_KEY_PATH"),
		GitHubOrganization:   os.Getenv("GITHUB_ORGANIZATION"),
		AppEnv:               os.Getenv("APP_ENV"),
		ModelTimeoutSeconds: getIntEnv(
			"MODEL_TIMEOUT_SECONDS",
			120,
		),
		APIToken: os.Getenv("API_TOKEN"),
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

func (c Config) Validate() error {
	if c.GitHubWebhookSecret == "" {
		return fmt.Errorf("GITHUB_WEBHOOK_SECRET is required")
	}

	if c.APIToken == "" {
		return fmt.Errorf("API_TOKEN is required")
	}

	if c.ModelBaseURL == "" {
		return fmt.Errorf("MODEL_BASE_URL is required")
	}

	if c.ModelName == "" {
		return fmt.Errorf("MODEL_NAME is required")
	}

	if c.ModelTimeoutSeconds <= 0 {
		return fmt.Errorf("MODEL_TIMEOUT_SECONDS must be greater than 0")
	}

	if c.GitHubOrganization == "" {
		return fmt.Errorf("GITHUB_ORGANIZATION is required")
	}

	if c.AppEnv != "development" {
		if c.GitHubAppID <= 0 {
			return fmt.Errorf("GITHUB_APP_ID must be greater than 0")
		}

		if c.GitHubPrivateKeyPath == "" {
			return fmt.Errorf("GITHUB_PRIVATE_KEY_PATH is required")
		}
	}

	return nil
}
