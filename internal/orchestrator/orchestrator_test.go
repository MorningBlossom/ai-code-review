package orchestrator

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/model"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
)

func TestReview_Success(t *testing.T) {
	orchestrator := NewOrchestrator(
		&github.FakeProvider{},
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{},
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	request := review.ReviewRequest{
		ReviewID:            "review-test-001",
		Organization:        "MorningBlossom",
		Repository:          "test-repo",
		PullRequestNumber:   10,
		BaseSHA:             "base-123",
		HeadSHA:             "head-456",
		EventType:           "pull_request",
		ReviewMode:          "full",
		ReviewPolicyVersion: "v1",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf(
			"expected status completed, got %s",
			result.Status,
		)
	}

	if result.ReviewID != request.ReviewID {
		t.Fatalf(
			"expected review ID %s, got %s",
			request.ReviewID,
			result.ReviewID,
		)
	}

	if len(result.Findings) != 2 {
		t.Fatalf(
			"expected 2 findings, got %d",
			len(result.Findings),
		)
	}

	if len(result.AnalyzerSummary) != 1 {
		t.Fatalf(
			"expected 1 analyzer result, got %d",
			len(result.AnalyzerSummary),
		)
	}
}

func TestReview_GitHubFailure(t *testing.T) {
	githubProvider := &github.FakeProvider{
		GetPullRequestError: github.ErrFakeGitHub,
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{},
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	request := review.ReviewRequest{
		ReviewID:          "review-test-002",
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 10,
		BaseSHA:           "base-123",
		HeadSHA:           "head-456",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %s",
			result.Status,
		)
	}

	if len(result.Errors) != 1 {
		t.Fatalf(
			"expected 1 error, got %d",
			len(result.Errors),
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings, got %d",
			len(result.Findings),
		)
	}
}

func TestReview_AnalyzerFailure(t *testing.T) {
	orchestrator := NewOrchestrator(
		&github.FakeProvider{},
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{
				Fail: true,
			},
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	request := review.ReviewRequest{
		ReviewID:          "review-test-003",
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 10,
		BaseSHA:           "base-123",
		HeadSHA:           "head-456",
	}

	result, err := orchestrator.Review(context.Background(), request)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("expected status completed, got %s", result.Status)
	}

	if len(result.AnalyzerSummary) != 1 {
		t.Fatalf(
			"expected 1 analyzer result, got %d",
			len(result.AnalyzerSummary),
		)
	}

	if result.AnalyzerSummary[0].Status != "failed" {
		t.Fatalf(
			"expected analyzer status failed, got %s",
			result.AnalyzerSummary[0].Status,
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected model finding to remain available, got %d findings",
			len(result.Findings),
		)
	}
}

func TestReview_ModelFailure(t *testing.T) {
	orchestrator := NewOrchestrator(
		&github.FakeProvider{},
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{},
		},
		&model.FakeModelProvider{
			Fail: true,
		},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	request := review.ReviewRequest{
		ReviewID:          "review-test-004",
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 10,
		BaseSHA:           "base-123",
		HeadSHA:           "head-456",
	}

	result, err := orchestrator.Review(context.Background(), request)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %s", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf(
			"expected 1 error, got %d",
			len(result.Errors),
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings, got %d",
			len(result.Findings),
		)
	}
}

func TestReview_ValidatorFailure(t *testing.T) {
	orchestrator := NewOrchestrator(
		&github.FakeProvider{},
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{},
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{
			Fail: true,
		},
		&publisher.FakePublisher{},
	)

	request := review.ReviewRequest{
		ReviewID:          "review-test-005",
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 10,
		BaseSHA:           "base-123",
		HeadSHA:           "head-456",
	}

	result, err := orchestrator.Review(context.Background(), request)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %s", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestReview_PublisherFailure(t *testing.T) {
	orchestrator := NewOrchestrator(
		&github.FakeProvider{},
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{},
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{
			Fail: true,
		},
	)

	request := review.ReviewRequest{
		ReviewID:          "review-test-006",
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 10,
		BaseSHA:           "base-123",
		HeadSHA:           "head-456",
	}

	result, err := orchestrator.Review(context.Background(), request)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %s", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}

	if len(result.Findings) != 2 {
		t.Fatalf(
			"expected findings to remain in result, got %d",
			len(result.Findings),
		)
	}
}
