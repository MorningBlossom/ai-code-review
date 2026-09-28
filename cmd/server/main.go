package main

import (
	"log"
	"net/http"

	"github.com/MorningBlossom/ai-code-review/config"
	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/api"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/model"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
)

func main() {

	cfg := config.Load()

	if cfg.GitHubWebhookSecret == "" {
		log.Fatal("GITHUB_WEBHOOK_SECRET is required")
	}

	if cfg.GitHubAppID == 0 {
		log.Fatal("GITHUB_APP_ID is required")
	}

	if cfg.GitHubPrivateKeyPath == "" {
		log.Fatal("GITHUB_PRIVATE_KEY_PATH is required")
	}

	appAuthenticator, err := github.NewGitHubAppAuthenticator(
		github.AppConfig{
			AppID:          cfg.GitHubAppID,
			PrivateKeyPath: cfg.GitHubPrivateKeyPath,
		},
	)
	if err != nil {
		log.Fatalf("create GitHub App authenticator: %v", err)
	}

	fakeAnalyzers := []analyzer.Analyzer{
		&analyzer.FakeAnalyzer{},
	}

	fakeModel := &model.FakeModelProvider{}

	fakeValidator := &validator.FakeValidator{}

	fakePublisher := &publisher.FakePublisher{}

	githubClient := github.NewClient(
		http.DefaultClient,
		appAuthenticator,
		"https://api.github.com",
	)

	deliveryStore := api.NewDeliveryStore()

	contextBuilder := contextbuilder.NewBuilder(
		githubClient,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := orchestrator.NewOrchestrator(
		githubClient,
		contextBuilder,
		fakeAnalyzers,
		fakeModel,
		fakeValidator,
		fakePublisher,
	)

	githubWebhookHandler := api.NewGithubWebhookHandler(reviewOrchestrator, cfg.GitHubWebhookSecret, deliveryStore)

	handler := api.NewHandler(reviewOrchestrator)
	http.HandleFunc("/health", handler.Health)
	http.HandleFunc(
		"/github/webhook",
		githubWebhookHandler.HandleWebhook,
	)
	http.HandleFunc("/reviews", handler.CreateReview)
	log.Println("review API listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
