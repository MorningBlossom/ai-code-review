package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type failingGitHubProvider struct {
	err error
}

func (p *failingGitHubProvider) GetPullRequest(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) (github.PullRequest, error) {
	return github.PullRequest{}, p.err
}

func (p *failingGitHubProvider) GetChangedFiles(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) ([]review.ChangedFile, error) {
	return nil, p.err
}

func (p *failingGitHubProvider) GetFileContent(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
	path string,
) (string, error) {
	return "", p.err
}

func (p *failingGitHubProvider) DownloadRepositoryArchive(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) ([]byte, error) {
	return nil, p.err
}

type failingModelProvider struct {
	err error
}

func (m *failingModelProvider) Name() string {
	return "failing-model"
}

func (m *failingModelProvider) Review(
	ctx context.Context,
	reviewContext review.ReviewContext,
) ([]review.ReviewFinding, error) {
	return nil, m.err
}

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
	err               error
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

func (v *fakeValidator) ValidateWithContext(
	ctx context.Context,
	request review.ReviewRequest,
	reviewContext review.ReviewContext,
	findings []review.ReviewFinding,
) ([]review.ReviewFinding, error) {
	v.contextCalls++

	if v.err != nil {
		return nil, v.err
	}

	v.lastReviewContext = reviewContext

	return findings, nil
}

type fakePublisher struct {
	calls int
	err   error
}

func (p *fakePublisher) Publish(
	ctx context.Context,
	request review.ReviewRequest,
	result review.ReviewResult,
) error {
	p.calls++

	if p.err != nil {
		return p.err
	}

	return nil
}

type fakeWorkspaceBuilder struct {
	calls int
	err   error
}

func (f *fakeWorkspaceBuilder) Build(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) (*workspace.TempWorkspace, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

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

func TestReviewOrchestrator_PublishesWhenWorkspaceAnalyzerFails(t *testing.T) {
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
		fn: func(ctx context.Context, ws workspace.Workspace) review.AnalyzerResult {
			return review.AnalyzerResult{
				AnalyzerName: "go-vet",
				Status:       "failed",
				Diagnostics:  []string{"go vet execution failed: go: command not found"},
			}
		},
	}

	orchestrator := NewOrchestrator(
		githubProvider, contextBuilder, workspaceBuilder, nil,
		[]analyzer.WorkspaceAnalyzer{failingWorkspaceAnalyzer},
		modelProvider, findingValidator, reviewPublisher,
	)

	result, err := orchestrator.Review(context.Background(), review.ReviewRequest{
		ReviewID: "review-failure", InstallationID: 123,
		Organization: "MorningBlossom", Repository: "test-repository",
		PullRequestNumber: 1, BaseSHA: "base-sha", HeadSHA: "head-sha",
		RequestedAt: time.Now(),
	})

	if err != nil {
		t.Fatalf("expected analyzer failure to be published, got %v", err)
	}
	if result.Status != "completed_with_analyzer_failures" {
		t.Fatalf("expected analyzer failure status, got %q", result.Status)
	}
	if len(result.AnalyzerSummary) != 1 {
		t.Fatalf("expected one analyzer result, got %d", len(result.AnalyzerSummary))
	}
	if modelProvider.calls != 0 {
		t.Fatalf("expected model not to be called, got %d calls", modelProvider.calls)
	}
	if findingValidator.contextCalls != 1 {
		t.Fatalf("expected context-aware validator to be called once, got %d calls", findingValidator.contextCalls)
	}
	if reviewPublisher.calls != 1 {
		t.Fatalf("expected publisher to be called once, got %d calls", reviewPublisher.calls)
	}
}
func TestReviewOrchestrator_ValidatesAnalyzerFindingsBeforePublish(t *testing.T) {
	githubProvider := &fakeGitHubProvider{}
	modelProvider := &fakeModelProvider{}
	reviewPublisher := &fakePublisher{}
	workspaceBuilder := &fakeWorkspaceBuilder{}

	contextBuilder := contextbuilder.NewBuilder(
		githubProvider,
		20,
		200_000,
		50_000,
	)

	failingAnalyzer := workspaceAnalyzerFunc{
		name: "go-vet",
		fn: func(ctx context.Context, ws workspace.Workspace) review.AnalyzerResult {
			return review.AnalyzerResult{
				AnalyzerName: "go-vet",
				Status:       "failed",
				Findings: []review.ReviewFinding{
					{
						FindingID:   "invalid-line",
						Source:      "go-vet",
						Category:    "correctness",
						Severity:    "high",
						Confidence:  1,
						Title:       "Invalid line location",
						Explanation: "This line is outside the supplied PR patch.",
						FilePath:    "main.go",
						StartLine:   10,
						EndLine:     10,
					},
				},
				Diagnostics: []string{"go vet failed"},
			}
		},
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextBuilder,
		workspaceBuilder,
		nil,
		[]analyzer.WorkspaceAnalyzer{failingAnalyzer},
		modelProvider,
		validator.NewFindingValidator(),
		reviewPublisher,
	)

	result, err := orchestrator.Review(
		context.Background(),
		review.ReviewRequest{
			ReviewID:          "review-analyzer-invalid-line",
			InstallationID:    123,
			Organization:      "MorningBlossom",
			Repository:        "test-repository",
			PullRequestNumber: 1,
			BaseSHA:           "base-sha",
			HeadSHA:           "head-sha",
		},
	)
	if err != nil {
		t.Fatalf("expected analyzer failure to be published, got %v", err)
	}

	if result.Status != "completed_with_analyzer_failures" {
		t.Fatalf("expected analyzer failure status, got %q", result.Status)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected invalid analyzer finding to be filtered before publishing, got %d findings",
			len(result.Findings),
		)
	}

	if modelProvider.calls != 0 {
		t.Fatalf("expected model not to be called, got %d calls", modelProvider.calls)
	}

	if reviewPublisher.calls != 1 {
		t.Fatalf("expected publisher to be called once, got %d calls", reviewPublisher.calls)
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

func TestReviewOrchestrator_FailsWhenModelFails(t *testing.T) {
	githubProvider := &fakeGitHubProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}
	workspaceBuilder := &fakeWorkspaceBuilder{}

	contextBuilder := contextbuilder.NewBuilder(
		githubProvider,
		20,
		200_000,
		50_000,
	)

	modelProvider := &failingModelProvider{
		err: errors.New("model provider failed"),
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextBuilder,
		workspaceBuilder,
		nil,
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-model-failure",
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
		t.Fatal("expected model failure error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected failed status, got %q",
			result.Status,
		)
	}

	if !strings.Contains(err.Error(), "model provider failed") {
		t.Fatalf(
			"expected model failure error, got %v",
			err,
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

func TestReviewOrchestrator_FailsWhenGetPullRequestFails(t *testing.T) {
	expectedErr := errors.New("github pull request failed")

	githubProvider := &failingGitHubProvider{
		err: expectedErr,
	}

	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextbuilder.NewBuilder(
			githubProvider,
			10,        // maxFiles
			1_000_000, // maxBytes
			100_000,   // maxTokens
		),
		nil,
		nil,
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-github-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "get pull request") {
		t.Fatalf("expected get pull request error, got %v", err)
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected underlying GitHub error, got %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected one result error, got %d", len(result.Errors))
	}

	if modelProvider.calls != 0 {
		t.Fatalf("expected model not to be called, got %d calls", modelProvider.calls)
	}

	if findingValidator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d calls", findingValidator.calls)
	}

	if reviewPublisher.calls != 0 {
		t.Fatalf("expected publisher not to be called, got %d calls", reviewPublisher.calls)
	}
}

type changedFilesFailingGitHubProvider struct {
	err error
}

func (p *changedFilesFailingGitHubProvider) GetPullRequest(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) (github.PullRequest, error) {
	return github.PullRequest{
		Number:  pullRequestNumber,
		Title:   "Test PR",
		Body:    "Test body",
		BaseSHA: "base-sha",
		HeadSHA: "head-sha",
		Author:  "test-user",
	}, nil
}

func (p *changedFilesFailingGitHubProvider) GetChangedFiles(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) ([]review.ChangedFile, error) {
	return nil, p.err
}

func (p *changedFilesFailingGitHubProvider) GetFileContent(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
	path string,
) (string, error) {
	return "", p.err
}

func (p *changedFilesFailingGitHubProvider) DownloadRepositoryArchive(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) ([]byte, error) {
	return nil, p.err
}

func TestReviewOrchestrator_FailsWhenGetChangedFilesFails(t *testing.T) {
	expectedErr := errors.New("github changed files failed")

	githubProvider := &changedFilesFailingGitHubProvider{
		err: expectedErr,
	}

	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextbuilder.NewBuilder(
			githubProvider,
			10,
			1_000_000,
			100_000,
		),
		nil,
		nil,
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-changed-files-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "get changed files") {
		t.Fatalf("expected get changed files error, got %v", err)
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected underlying GitHub error, got %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected one result error, got %d", len(result.Errors))
	}

	if modelProvider.calls != 0 {
		t.Fatalf("expected model not to be called, got %d calls", modelProvider.calls)
	}

	if findingValidator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d calls", findingValidator.calls)
	}

	if reviewPublisher.calls != 0 {
		t.Fatalf("expected publisher not to be called, got %d calls", reviewPublisher.calls)
	}
}

type contextBuildFailingGitHubProvider struct {
	err error
}

func (p *contextBuildFailingGitHubProvider) GetPullRequest(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) (github.PullRequest, error) {
	return github.PullRequest{
		Number:  pullRequestNumber,
		Title:   "Test PR",
		Body:    "Test body",
		BaseSHA: "base-sha",
		HeadSHA: "head-sha",
		Author:  "test-user",
	}, nil
}

func (p *contextBuildFailingGitHubProvider) GetChangedFiles(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) ([]review.ChangedFile, error) {
	return []review.ChangedFile{
		{
			Path:   "internal/example.go",
			Status: "modified",
		},
	}, nil
}

func (p *contextBuildFailingGitHubProvider) GetFileContent(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
	path string,
) (string, error) {
	return "", p.err
}

func (p *contextBuildFailingGitHubProvider) DownloadRepositoryArchive(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) ([]byte, error) {
	return nil, p.err
}

func TestReviewOrchestrator_FailsWhenContextBuildFails(t *testing.T) {
	expectedErr := errors.New("github source file failed")

	githubProvider := &contextBuildFailingGitHubProvider{
		err: expectedErr,
	}

	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextbuilder.NewBuilder(
			githubProvider,
			10,
			1_000_000,
			100_000,
		),
		nil,
		nil,
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-context-build-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "build review context") {
		t.Fatalf("expected build review context error, got %v", err)
	}

	if !strings.Contains(err.Error(), "get source file internal/example.go") {
		t.Fatalf("expected source file error, got %v", err)
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected underlying GitHub error, got %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected one result error, got %d", len(result.Errors))
	}

	if modelProvider.calls != 0 {
		t.Fatalf("expected model not to be called, got %d calls", modelProvider.calls)
	}

	if findingValidator.calls != 0 {
		t.Fatalf("expected validator not to be called, got %d calls", findingValidator.calls)
	}

	if reviewPublisher.calls != 0 {
		t.Fatalf("expected publisher not to be called, got %d calls", reviewPublisher.calls)
	}
}

func TestReviewOrchestrator_FailsWhenValidatorFails(t *testing.T) {
	expectedErr := errors.New("finding validation failed")

	githubProvider := &fakeGitHubProvider{}

	modelProvider := &fakeModelProvider{}

	findingValidator := &fakeValidator{
		err: expectedErr,
	}

	reviewPublisher := &fakePublisher{}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextbuilder.NewBuilder(
			githubProvider,
			10,
			1_000_000,
			100_000,
		),
		nil,
		nil,
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-validator-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "validate findings") {
		t.Fatalf("expected validate findings error, got %v", err)
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected underlying validator error, got %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected one result error, got %d", len(result.Errors))
	}

	if modelProvider.calls != 1 {
		t.Fatalf("expected model to be called once, got %d calls", modelProvider.calls)
	}

	if reviewPublisher.calls != 0 {
		t.Fatalf("expected publisher not to be called, got %d calls", reviewPublisher.calls)
	}
}

func TestReviewOrchestrator_FailsWhenPublisherFails(t *testing.T) {
	expectedErr := errors.New("publisher failed")

	githubProvider := &fakeGitHubProvider{}

	modelProvider := &fakeModelProvider{}

	findingValidator := &fakeValidator{}

	reviewPublisher := &fakePublisher{
		err: expectedErr,
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextbuilder.NewBuilder(
			githubProvider,
			10,
			1_000_000,
			100_000,
		),
		nil,
		nil,
		nil,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-publisher-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "publish review") {
		t.Fatalf("expected publish review error, got %v", err)
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected underlying publisher error, got %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected one result error, got %d", len(result.Errors))
	}

	if reviewPublisher.calls != 1 {
		t.Fatalf(
			"expected publisher to be called once, got %d",
			reviewPublisher.calls,
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
			"expected validator to be called once, got %d",
			findingValidator.contextCalls,
		)
	}
}

func TestReviewOrchestrator_FailsWhenWorkspaceBuildFails(t *testing.T) {
	expectedErr := errors.New("workspace build failed")

	githubProvider := &fakeGitHubProvider{}

	workspaceBuilder := &fakeWorkspaceBuilder{
		err: expectedErr,
	}

	modelProvider := &fakeModelProvider{}
	findingValidator := &fakeValidator{}
	reviewPublisher := &fakePublisher{}

	workspaceAnalyzer := &fakeWorkspaceAnalyzer{
		name: "go-vet",
	}

	orchestrator := NewOrchestrator(
		githubProvider,
		contextbuilder.NewBuilder(
			githubProvider,
			10,
			1_000_000,
			100_000,
		),
		workspaceBuilder,
		nil,
		[]analyzer.WorkspaceAnalyzer{
			workspaceAnalyzer,
		},
		modelProvider,
		findingValidator,
		reviewPublisher,
	)

	request := review.ReviewRequest{
		ReviewID:          "review-workspace-build-failure",
		InstallationID:    123,
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 42,
		BaseSHA:           "base-sha",
		HeadSHA:           "head-sha",
	}

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "build repository workspace") {
		t.Fatalf(
			"expected build repository workspace error, got %v",
			err,
		)
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf(
			"expected underlying workspace error, got %v",
			err,
		)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Errors) != 1 {
		t.Fatalf(
			"expected one result error, got %d",
			len(result.Errors),
		)
	}

	if workspaceBuilder.calls != 1 {
		t.Fatalf(
			"expected workspace builder to be called once, got %d",
			workspaceBuilder.calls,
		)
	}

	if workspaceAnalyzer.calls != 0 {
		t.Fatalf(
			"expected workspace analyzer not to be called, got %d calls",
			workspaceAnalyzer.calls,
		)
	}

	if modelProvider.calls != 0 {
		t.Fatalf(
			"expected model not to be called, got %d calls",
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
