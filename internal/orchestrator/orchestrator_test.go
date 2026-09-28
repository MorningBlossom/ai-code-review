package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
)

type contextCaptureModel struct {
	called   bool
	context  review.ReviewContext
	findings []review.ReviewFinding
	err      error
}

func (m *contextCaptureModel) Name() string {
	return "context-capture-model"
}

func (m *contextCaptureModel) Review(
	ctx context.Context,
	reviewContext review.ReviewContext,
) ([]review.ReviewFinding, error) {
	m.called = true
	m.context = reviewContext

	if m.err != nil {
		return nil, m.err
	}

	return m.findings, nil
}

type contextCapturePublisher struct {
	called bool
	result review.ReviewResult
}

func (p *contextCapturePublisher) Publish(
	ctx context.Context,
	request review.ReviewRequest,
	result review.ReviewResult,
) error {
	p.called = true
	p.result = result
	return nil
}

func TestReviewOrchestrator_Success(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	analyzers := []analyzer.Analyzer{
		&analyzer.FakeAnalyzer{},
	}

	modelProvider := &contextCaptureModel{
		findings: []review.ReviewFinding{
			{
				FindingID:   "model-1",
				Source:      "model",
				Category:    "performance",
				Severity:    "low",
				Confidence:  0.9,
				Title:       "Potential performance issue",
				Explanation: "This may perform unnecessary work.",
				FilePath:    "payment/retry.go",
				StartLine:   3,
				EndLine:     3,
			},
		},
	}

	fakeValidator := &validator.FakeValidator{}

	publisher := &contextCapturePublisher{}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		analyzers,
		modelProvider,
		fakeValidator,
		publisher,
	)

	request := review.ReviewRequest{
		ReviewID:            "review-123",
		InstallationID:      12345,
		Organization:        "MorningBlossom",
		Repository:          "test-repo",
		PullRequestNumber:   42,
		BaseSHA:             "base-123",
		HeadSHA:             "head-456",
		EventType:           "pull_request",
		RequestedBy:         "test-user",
		ReviewMode:          "pull_request",
		ReviewPolicyVersion: "v1",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("expected status completed, got %q", result.Status)
	}

	if result.ReviewID != request.ReviewID {
		t.Fatalf(
			"expected review ID %q, got %q",
			request.ReviewID,
			result.ReviewID,
		)
	}

	if result.Organization != request.Organization {
		t.Fatalf(
			"expected organization %q, got %q",
			request.Organization,
			result.Organization,
		)
	}

	if result.Repository != request.Repository {
		t.Fatalf(
			"expected repository %q, got %q",
			request.Repository,
			result.Repository,
		)
	}

	if result.PullRequestNumber != request.PullRequestNumber {
		t.Fatalf(
			"expected pull request number %d, got %d",
			request.PullRequestNumber,
			result.PullRequestNumber,
		)
	}

	if result.BaseSHA != "base-123" {
		t.Fatalf("expected base SHA base-123, got %q", result.BaseSHA)
	}

	if result.HeadSHA != "head-456" {
		t.Fatalf("expected head SHA head-456, got %q", result.HeadSHA)
	}

	if !modelProvider.called {
		t.Fatal("expected model provider to be called")
	}

	if !publisher.called {
		t.Fatal("expected publisher to be called")
	}

	if modelProvider.context.PullRequestTitle != "Add payment retry handling" {
		t.Fatalf(
			"expected PR title %q, got %q",
			"Add payment retry handling",
			modelProvider.context.PullRequestTitle,
		)
	}

	if modelProvider.context.PullRequestBody == "" {
		t.Fatal("expected PR body to be present in review context")
	}

	if len(modelProvider.context.ChangedFiles) != 1 {
		t.Fatalf(
			"expected 1 changed file, got %d",
			len(modelProvider.context.ChangedFiles),
		)
	}

	if len(modelProvider.context.SourceFiles) != 1 {
		t.Fatalf(
			"expected 1 source file, got %d",
			len(modelProvider.context.SourceFiles),
		)
	}

	if modelProvider.context.SourceFiles[0].Path != "payment/retry.go" {
		t.Fatalf(
			"expected source file payment/retry.go, got %q",
			modelProvider.context.SourceFiles[0].Path,
		)
	}

	if len(result.Findings) != 2 {
		t.Fatalf(
			"expected 2 findings, got %d",
			len(result.Findings),
		)
	}

	if publisher.result.ReviewID != request.ReviewID {
		t.Fatalf(
			"publisher received review ID %q, expected %q",
			publisher.result.ReviewID,
			request.ReviewID,
		)
	}
}

func TestReviewOrchestrator_GitHubPullRequestError(t *testing.T) {
	expectedErr := errors.New("github pull request error")

	fakeGitHub := &github.FakeProvider{
		GetPullRequestError: expectedErr,
	}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		nil,
		nil,
		nil,
		nil,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-error",
		InstallationID:    12345,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected wrapped github error, got %v",
			err,
		)
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}
}

func TestReviewOrchestrator_ChangedFilesError(t *testing.T) {
	expectedErr := errors.New("github changed files error")

	fakeGitHub := &github.FakeProvider{
		GetChangedFilesError: expectedErr,
	}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		nil,
		nil,
		nil,
		nil,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-error",
		InstallationID:    12345,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected wrapped changed-files error, got %v",
			err,
		)
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}
}

func TestReviewOrchestrator_AnalyzerFailureIsolated(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	failingAnalyzer := &analyzer.FakeAnalyzer{
		Fail: true,
	}

	modelProvider := &contextCaptureModel{}

	fakeValidator := &validator.FakeValidator{}

	publisher := &contextCapturePublisher{}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		[]analyzer.Analyzer{
			failingAnalyzer,
		},
		modelProvider,
		fakeValidator,
		publisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-analyzer-failure",
		InstallationID:    12345,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf(
			"expected status completed despite analyzer failure, got %q",
			result.Status,
		)
	}

	if len(result.AnalyzerSummary) != 1 {
		t.Fatalf(
			"expected 1 analyzer result, got %d",
			len(result.AnalyzerSummary),
		)
	}

	if result.AnalyzerSummary[0].Status != "failed" {
		t.Fatalf(
			"expected analyzer status failed, got %q",
			result.AnalyzerSummary[0].Status,
		)
	}

	if !modelProvider.called {
		t.Fatal("expected model provider to still be called")
	}

	if !publisher.called {
		t.Fatal("expected publisher to still be called")
	}
}

func TestReviewOrchestrator_ModelFailure(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	modelErr := errors.New("model provider error")

	modelProvider := &contextCaptureModel{
		err: modelErr,
	}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		nil,
		modelProvider,
		nil,
		nil,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-model-failure",
		InstallationID:    12345,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, modelErr) {
		t.Fatalf(
			"expected wrapped model error, got %v",
			err,
		)
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}
}

func TestReviewOrchestrator_ValidatorFailure(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	fakeValidator := &validator.FakeValidator{
		Fail: true,
	}

	modelProvider := &contextCaptureModel{
		findings: []review.ReviewFinding{
			{
				FindingID: "model-1",
				Source:    "model",
				Category:  "bug",
				Severity:  "medium",
				Title:     "Potential bug",
				FilePath:  "payment/retry.go",
				StartLine: 3,
				EndLine:   3,
			},
		},
	}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		nil,
		modelProvider,
		fakeValidator,
		nil,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-validator-failure",
		InstallationID:    12345,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err == nil {
		t.Fatal("expected error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}
}

func TestReviewOrchestrator_PublisherFailure(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	fakePublisher := &publisher.FakePublisher{
		Fail: true,
	}

	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	reviewOrchestrator := NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		nil,
		&contextCaptureModel{},
		&validator.FakeValidator{},
		fakePublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-publisher-failure",
		InstallationID:    12345,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		HeadSHA:           "head-456",
	}

	result, err := reviewOrchestrator.Review(context.Background(), request)
	if err == nil {
		t.Fatal("expected error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}
}
