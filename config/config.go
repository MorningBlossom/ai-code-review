package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	GitHubWebhookSecret  string
	GitHubAppID          int64
	GitHubPrivateKeyPath string
}

func Load() Config {

	_ = godotenv.Load()

	return Config{
		GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
	}
}
