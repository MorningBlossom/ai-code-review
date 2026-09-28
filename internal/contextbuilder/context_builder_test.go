package contextbuilder

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestBuilderBuildsContext(t *testing.T) {
	githubProvider := &github.FakeProvider{}

	builder := NewBuilder(
		githubProvider,
		10,
		10_000,
		10_000,
	)

	request := review.ReviewRequest{
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
		BaseSHA:           "base-123",
	}

	pr := github.PullRequest{
		Number:  42,
		Title:   "Add payment retry handling",
		Body:    "This PR adds retry handling for failed payments.",
		BaseSHA: "base-123",
		HeadSHA: "head-456",
		Author:  "test-user",
	}

	changedFiles := []review.ChangedFile{
		{
			Path:      "payment/retry.go",
			Status:    "modified",
			Additions: 20,
			Deletions: 5,
		},
	}

	result, err := builder.Build(
		context.Background(),
		request,
		pr,
		changedFiles,
	)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if result.PullRequestTitle != pr.Title {
		t.Fatalf("unexpected title: %s", result.PullRequestTitle)
	}

	if result.PullRequestBody != pr.Body {
		t.Fatalf("unexpected body: %s", result.PullRequestBody)
	}

	if len(result.SourceFiles) != 1 {
		t.Fatalf(
			"expected 1 source file, got %d",
			len(result.SourceFiles),
		)
	}

	if result.SourceFiles[0].Path != "payment/retry.go" {
		t.Fatalf(
			"unexpected source file: %s",
			result.SourceFiles[0].Path,
		)
	}

	if result.SourceFiles[0].Content == "" {
		t.Fatal("expected source file content")
	}

	if result.ContextBudget.UsedBytes <= 0 {
		t.Fatal("expected used bytes to be greater than zero")
	}

	if result.ContextBudget.EstimatedTokens <= 0 {
		t.Fatal("expected estimated tokens to be greater than zero")
	}
}

func TestBuilderRespectsMaxFiles(t *testing.T) {
	githubProvider := &github.FakeProvider{}

	builder := NewBuilder(
		githubProvider,
		1,
		10_000,
		10_000,
	)

	request := review.ReviewRequest{
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
		BaseSHA:           "base-123",
	}

	pr := github.PullRequest{
		Number:  42,
		Title:   "Test PR",
		Body:    "Test body",
		BaseSHA: "base-123",
		HeadSHA: "head-456",
	}

	changedFiles := []review.ChangedFile{
		{
			Path:   "payment/retry.go",
			Status: "modified",
		},
		{
			Path:   "payment/retry_test.go",
			Status: "modified",
		},
	}

	result, err := builder.Build(
		context.Background(),
		request,
		pr,
		changedFiles,
	)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if len(result.SourceFiles) != 1 {
		t.Fatalf(
			"expected 1 source file, got %d",
			len(result.SourceFiles),
		)
	}
}

func TestBuilderRespectsMaxBytes(t *testing.T) {
	githubProvider := &github.FakeProvider{}

	builder := NewBuilder(
		githubProvider,
		10,
		10,
		10_000,
	)

	request := review.ReviewRequest{
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
		BaseSHA:           "base-123",
	}

	pr := github.PullRequest{
		Number:  42,
		Title:   "Test PR",
		Body:    "Test body",
		BaseSHA: "base-123",
		HeadSHA: "head-456",
	}

	changedFiles := []review.ChangedFile{
		{
			Path:   "payment/retry.go",
			Status: "modified",
		},
	}

	result, err := builder.Build(
		context.Background(),
		request,
		pr,
		changedFiles,
	)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if len(result.SourceFiles) != 0 {
		t.Fatalf(
			"expected no source files due to byte limit, got %d",
			len(result.SourceFiles),
		)
	}
}
