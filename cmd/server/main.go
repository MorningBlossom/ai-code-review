package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	if err := appConfig.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	var githubClient github.Provider
	var githubReviewClient publisher.GitHubReviewClient

	if appConfig.AppEnv == "development" {
		archive, err := github.NewFakeRepositoryArchive()
		if err != nil {
			log.Fatalf("failed to create fake repository archive: %v", err)
		}

		fakeGitHub := &github.FakeProvider{
			Archive: archive,
		}

		githubClient = fakeGitHub

		// Keep fake publishing in development.
		githubReviewClient = nil
	} else {

		appAuthenticator, err := github.NewGitHubAppAuthenticator(
			github.AppConfig{
				AppID:          appConfig.GitHubAppID,
				PrivateKeyPath: appConfig.GitHubPrivateKeyPath,
			},
		)
		if err != nil {
			log.Fatalf("failed to create GitHub App authenticator: %v", err)
		}

		githubHTTPClient := &http.Client{
			Timeout: 30 * time.Second,
		}

		realGitHubClient := github.NewClient(
			githubHTTPClient,
			appAuthenticator,
			"https://api.github.com",
		)

		githubClient = realGitHubClient
		githubReviewClient = realGitHubClient
	}

	commandRunner := analyzer.NewOSCommandRunner()

	analyzers := []analyzer.Analyzer{
		analyzer.NewGofmtAnalyzer(commandRunner),
		analyzer.NewGoRulesAnalyzer(),
		analyzer.NewSecurityRulesAnalyzer(),
		analyzer.NewTestQualityAnalyzer(),
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

	findingValidator := validator.NewFindingValidator()

	var reviewPublisher publisher.Publisher

	if appConfig.AppEnv == "development" {
		reviewPublisher = &publisher.FakePublisher{}
	} else {
		reviewPublisher = publisher.NewGitHubPublisher(githubReviewClient)
	}

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
		findingValidator,
		reviewPublisher,
	)

	githubWebhookHandler := api.NewGithubWebhookHandler(
		reviewOrchestrator,
		appConfig.GitHubWebhookSecret,
		deliveryStore,
		appConfig.GitHubOrganization,
	)

	handler := api.NewHandler(
		reviewOrchestrator,
		appConfig.APIToken,
		appConfig.GitHubOrganization,
	)

	// routes
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/github/webhook", githubWebhookHandler.HandleWebhook)
	mux.HandleFunc("/reviews", handler.CreateReview)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Println("review API listening on :8080")

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignals := make(chan os.Signal, 1)

	signal.Notify(
		shutdownSignals,
		os.Interrupt,
		syscall.SIGTERM,
	)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}

	case sig := <-shutdownSignals:
		log.Printf("shutdown signal received: %s", sig)
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("graceful shutdown failed: %v", err)

		if err := server.Close(); err != nil {
			log.Printf("server close failed: %v", err)
		}
	}

	log.Println("review API stopped")
}
