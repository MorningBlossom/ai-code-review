package main

import (
	"log"
	"net/http"
	"time"

	"github.com/MorningBlossom/ai-code-review/config"
	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/api"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/model"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func main() {

	appConfig := config.Load()

	var githubClient github.Provider

	if appConfig.AppEnv == "development" {
		archive, err := github.NewFakeRepositoryArchive()
		if err != nil {
			log.Fatalf("create fake repository archive: %v", err)
		}

		githubClient = &github.FakeProvider{
			Archive: archive,
		}
	} else {
		if appConfig.GitHubWebhookSecret == "" {
			log.Fatal("GITHUB_WEBHOOK_SECRET is required")
		}

		if appConfig.GitHubAppID == 0 {
			log.Fatal("GITHUB_APP_ID is required")
		}

		if appConfig.GitHubPrivateKeyPath == "" {
			log.Fatal("GITHUB_PRIVATE_KEY_PATH is required")
		}

		appAuthenticator, err := github.NewGitHubAppAuthenticator(
			github.AppConfig{
				AppID:          appConfig.GitHubAppID,
				PrivateKeyPath: appConfig.GitHubPrivateKeyPath,
			},
		)
		if err != nil {
			log.Fatalf("create GitHub App authenticator: %v", err)
		}

		githubClient = github.NewClient(
			http.DefaultClient,
			appAuthenticator,
			"https://api.github.com",
		)
	}

	commandRunner := analyzer.NewOSCommandRunner()

	analyzers := []analyzer.Analyzer{
		analyzer.NewGofmtAnalyzer(commandRunner),
	}

	workspaceAnalyzers := []analyzer.WorkspaceAnalyzer{
		analyzer.NewGoVetAnalyzer(),
		analyzer.NewGoTestAnalyzer(),
		analyzer.NewStaticcheckAnalyzer(),
		analyzer.NewGosecAnalyzer(),
		analyzer.NewGovulncheckAnalyzer(),
		analyzer.NewGoRaceAnalyzer(),
	}

	modelProvider := model.NewLocalProvider(
		model.Config{
			BaseURL:        appConfig.ModelBaseURL,
			Model:          appConfig.ModelName,
			TimeoutSeconds: appConfig.ModelTimeoutSeconds,
		},
	)

	fakeValidator := &validator.FakeValidator{}

	fakePublisher := &publisher.FakePublisher{}

	deliveryStore := api.NewDeliveryStore()

	contextBuilder := contextbuilder.NewBuilder(
		githubClient,
		20,
		200_000,
		50_000,
	)

	workspaceBuilder := workspace.NewSnapshotBuilder(
		githubClient,
	)

	reviewOrchestrator := orchestrator.NewOrchestrator(
		githubClient,
		contextBuilder,
		workspaceBuilder,
		analyzers,
		workspaceAnalyzers,
		modelProvider,
		fakeValidator,
		fakePublisher,
	)

	githubWebhookHandler := api.NewGithubWebhookHandler(reviewOrchestrator, appConfig.GitHubWebhookSecret, deliveryStore)

	handler := api.NewHandler(reviewOrchestrator)

	// routes
	http.HandleFunc("/health", handler.Health)
	http.HandleFunc(
		"/github/webhook",
		githubWebhookHandler.HandleWebhook,
	)
	http.HandleFunc("/reviews", handler.CreateReview)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           nil,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Println("review API listening on :8080")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
