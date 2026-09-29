package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type fakeModelProvider struct {
	calls         int
	reviewContext review.ReviewContext
}

func (m *fakeModelProvider) Name() string {
	return "fake-model"
}

func (m *fakeModelProvider) Review(
	ctx context.Context,
	reviewContext review.ReviewContext,
) ([]review.ReviewFinding, error) {
	m.calls++
	m.reviewContext = reviewContext

	return nil, nil
}

type fakeAnalyzer struct {
	name     string
	calls    int
	findings []review.ReviewFinding
}

func (a *fakeAnalyzer) Name() string {
	return a.name
}

func (a *fakeAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	a.calls++

	return review.AnalyzerResult{
		AnalyzerName: a.name,
		Status:       "passed",
		Findings:     a.findings,
	}
}

type workspaceAnalyzerFunc struct {
	name string
	fn   func(
		ctx context.Context,
		ws workspace.Workspace,
	) review.AnalyzerResult
}

func (a workspaceAnalyzerFunc) Name() string {
	return a.name
}

func (a workspaceAnalyzerFunc) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	return a.fn(ctx, ws)
}

type fakeWorkspaceAnalyzer struct {
	name  string
	calls int
}

func (a *fakeWorkspaceAnalyzer) Name() string {
	return a.name
}

func (a *fakeWorkspaceAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	a.calls++

	return review.AnalyzerResult{
		AnalyzerName: a.name,
		Status:       "passed",
	}
}

type fakeGitHubProvider struct{}

func (f *fakeGitHubProvider) GetPullRequest(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) (github.PullRequest, error) {
	return github.PullRequest{
		Number:  pullRequestNumber,
		Title:   "Test pull request",
		Body:    "Test pull request body",
		BaseSHA: "base-sha",
		HeadSHA: "head-sha",
		Author:  "test-user",
	}, nil
}

func (f *fakeGitHubProvider) GetChangedFiles(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) ([]review.ChangedFile, error) {
	return []review.ChangedFile{
		{
			Path:      "main.go",
			Status:    "modified",
			Additions: 1,
			Deletions: 0,
			Patch:     "@@ -1 +1 @@",
		},
	}, nil
}

func (f *fakeGitHubProvider) GetFileContent(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
	path string,
) (string, error) {
	return "package main\n\nfunc main() {}\n", nil
}

func (f *fakeGitHubProvider) DownloadRepositoryArchive(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) ([]byte, error) {
	return nil, nil
}

type fakeValidator struct {
	calls             int
	contextCalls      int
	lastReviewContext review.ReviewContext
}

func (f *fakeValidator) Validate(
	ctx context.Context,
	request review.ReviewRequest,
	findings []review.ReviewFinding,
) ([]review.ReviewFinding, error) {
	f.calls++

	return findings, nil
}

func (f *fakeValidator) ValidateWithContext(
	ctx context.Context,
	request review.ReviewRequest,
	reviewContext review.ReviewContext,
	findings []review.ReviewFinding,
) ([]review.ReviewFinding, error) {
	f.contextCalls++
	f.lastReviewContext = reviewContext

	return findings, nil
}

type fakePublisher struct {
	calls int
}

func (f *fakePublisher) Publish(
	ctx context.Context,
	request review.ReviewRequest,
	result review.ReviewResult,
) error {
	f.calls++

	return nil
}

type fakeWorkspaceBuilder struct {
	calls int
}

func (f *fakeWorkspaceBuilder) Build(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) (*workspace.TempWorkspace, error) {
	f.calls++

	return workspace.NewTempWorkspace()
}

func TestReviewOrchestrator_RunsAllWorkspaceAnalyzers(t *testing.T) {
	githubProvider := &fakeGitHubProvider{}
	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}
	workspaceBuilder := &fakeWorkspaceBuilder{}

	contextBuilder := contextbuilder.NewBuilder(
		githubProvider,
		20,
		200_000,
		50_000,
	)

	contextAnalyzers := []*fakeAnalyzer{
		{name: "gofmt"},
	}

	workspaceAnalyzers := []*fakeWorkspaceAnalyzer{
		{name: "go-vet"},
		{name: "go-test"},
		{name: "staticcheck"},
		{name: "gosec"},
		{name: "govulncheck"},
		{name: "go-race"},
	}

	contextAnalyzerInterfaces := make(
		[]analyzer.Analyzer,
		len(contextAnalyzers),
	)

	for i, currentAnalyzer := range contextAnalyzers {
		contextAnalyzerInterfaces[i] = currentAnalyzer
	}

	workspaceAnalyzerInterfaces := make(
		[]analyzer.WorkspaceAnalyzer,
		len(workspaceAnalyzers),
	)

	for i, currentAnalyzer := range workspaceAnalyzers {
		workspaceAnalyzerInterfaces[i] = currentAnalyzer
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextBuilder,
		workspaceBuilder,
		contextAnalyzerInterfaces,
		workspaceAnalyzerInterfaces,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:            "review-001",
		InstallationID:      123,
		Organization:        "MorningBlossom",
		Repository:          "test-repository",
		PullRequestNumber:   1,
		BaseSHA:             "base-sha",
		HeadSHA:             "head-sha",
		EventType:           "pull_request",
		RequestedBy:         "test-user",
		RequestedAt:         time.Now(),
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
			"expected completed status, got %q",
			result.Status,
		)
	}

	if workspaceBuilder.calls != 1 {
		t.Fatalf(
			"expected workspace builder to be called once, got %d",
			workspaceBuilder.calls,
		)
	}

	for _, currentAnalyzer := range contextAnalyzers {
		if currentAnalyzer.calls != 1 {
			t.Fatalf(
				"expected analyzer %q to be called once, got %d",
				currentAnalyzer.name,
				currentAnalyzer.calls,
			)
		}
	}

	for _, currentAnalyzer := range workspaceAnalyzers {
		if currentAnalyzer.calls != 1 {
			t.Fatalf(
				"expected workspace analyzer %q to be called once, got %d",
				currentAnalyzer.name,
				currentAnalyzer.calls,
			)
		}
	}

	expectedAnalyzerCount :=
		len(contextAnalyzers) +
			len(workspaceAnalyzers)

	if len(result.AnalyzerSummary) != expectedAnalyzerCount {
		t.Fatalf(
			"expected %d analyzer results, got %d",
			expectedAnalyzerCount,
			len(result.AnalyzerSummary),
		)
	}

	if modelProvider.calls != 1 {
		t.Fatalf(
			"expected model provider to be called once, got %d",
			modelProvider.calls,
		)
	}

	if findingValidator.contextCalls != 1 {
		t.Fatalf(
			"expected context-aware validator to be called once, got %d",
			findingValidator.contextCalls,
		)
	}

	if findingValidator.calls != 0 {
		t.Fatalf(
			"expected legacy validator not to be called, got %d calls",
			findingValidator.calls,
		)
	}

	if reviewPublisher.calls != 1 {
		t.Fatalf(
			"expected publisher to be called once, got %d",
			reviewPublisher.calls,
		)
	}
}

func TestReviewOrchestrator_FailsWhenWorkspaceAnalyzerFails(
	t *testing.T,
) {
	githubProvider := &fakeGitHubProvider{}
	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}
	workspaceBuilder := &fakeWorkspaceBuilder{}

	contextBuilder := contextbuilder.NewBuilder(
		githubProvider,
		20,
		200_000,
		50_000,
	)

	failingWorkspaceAnalyzer := workspaceAnalyzerFunc{
		name: "go-vet",
		fn: func(
			ctx context.Context,
			ws workspace.Workspace,
		) review.AnalyzerResult {
			return review.AnalyzerResult{
				AnalyzerName: "go-vet",
				Status:       "failed",
				Diagnostics: []string{
					"go vet execution failed",
				},
			}
		},
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextBuilder,
		workspaceBuilder,
		nil,
		[]analyzer.WorkspaceAnalyzer{
			failingWorkspaceAnalyzer,
		},
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repository",
		PullRequestNumber: 1,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
		RequestedAt:       time.Now(),
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected analyzer failure error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected failed status, got %q",
			result.Status,
		)
	}

	if len(result.AnalyzerSummary) != 1 {
		t.Fatalf(
			"expected one analyzer result, got %d",
			len(result.AnalyzerSummary),
		)
	}

	if modelProvider.calls != 0 {
		t.Fatalf(
			"expected model provider not to be called, got %d calls",
			modelProvider.calls,
		)
	}

	if findingValidator.contextCalls != 0 {
		t.Fatalf(
			"expected validator not to be called, got %d calls",
			findingValidator.contextCalls,
		)
	}

	if reviewPublisher.calls != 0 {
		t.Fatalf(
			"expected publisher not to be called, got %d calls",
			reviewPublisher.calls,
		)
	}
}

func TestReviewOrchestrator_PassesAnalyzerFindingsToModel(
	t *testing.T,
) {
	analyzerFinding := review.ReviewFinding{
		FindingID:   "test-finding-001",
		Source:      "test-analyzer",
		Category:    "security",
		Severity:    "high",
		Confidence:  0.95,
		Title:       "Potential security issue",
		Explanation: "Test explanation",
		Suggestion:  "Fix the issue",
		FilePath:    "main.go",
		StartLine:   10,
		EndLine:     10,
		Evidence:    "test evidence",
	}

	githubProvider := &fakeGitHubProvider{}
	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}
	workspaceBuilder := &fakeWorkspaceBuilder{}

	contextBuilder := contextbuilder.NewBuilder(
		githubProvider,
		20,
		200_000,
		50_000,
	)

	contextAnalyzer := &fakeAnalyzer{
		name: "test-analyzer",
		findings: []review.ReviewFinding{
			analyzerFinding,
		},
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextBuilder,
		workspaceBuilder,
		[]analyzer.Analyzer{
			contextAnalyzer,
		},
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-analyzer-findings",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repository",
		PullRequestNumber: 1,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
		RequestedAt:       time.Now(),
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
			"expected completed status, got %q",
			result.Status,
		)
	}

	if modelProvider.calls != 1 {
		t.Fatalf(
			"expected model provider to be called once, got %d",
			modelProvider.calls,
		)
	}

	if len(modelProvider.reviewContext.AnalyzerFindings) != 1 {
		t.Fatalf(
			"expected 1 analyzer finding passed to model, got %d",
			len(modelProvider.reviewContext.AnalyzerFindings),
		)
	}

	finding := modelProvider.reviewContext.AnalyzerFindings[0]

	if finding.FindingID != "test-finding-001" {
		t.Fatalf(
			"expected finding ID %q, got %q",
			"test-finding-001",
			finding.FindingID,
		)
	}

	if finding.Source != "test-analyzer" {
		t.Fatalf(
			"expected finding source %q, got %q",
			"test-analyzer",
			finding.Source,
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected 1 result finding, got %d",
			len(result.Findings),
		)
	}
}
