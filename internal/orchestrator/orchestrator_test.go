package orchestrator

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/model"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type contextCaptureModel struct {
	receivedContext review.ReviewContext
}

func (m *contextCaptureModel) Name() string {
	return "context-capture-model"
}

func (m *contextCaptureModel) Review(
	ctx context.Context,
	reviewContext review.ReviewContext,
) ([]review.ReviewFinding, error) {
	m.receivedContext = reviewContext

	return []review.ReviewFinding{
		{
			FindingID:   "model:001",
			Source:      "context-capture-model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.9,
			Title:       "Model finding",
			Explanation: "Test model finding.",
			FilePath:    "payment/retry.go",
			StartLine:   1,
			EndLine:     1,
		},
	}, nil
}

type contextCapturePublisher struct {
	receivedResult review.ReviewResult
}

func (p *contextCapturePublisher) Publish(
	ctx context.Context,
	request review.ReviewRequest,
	result review.ReviewResult,
) error {
	p.receivedResult = result
	return nil
}

type fakeWorkspaceAnalyzer struct {
	fail bool
}

func (a *fakeWorkspaceAnalyzer) Name() string {
	return "fake-workspace-analyzer"
}

func (a *fakeWorkspaceAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	if a.fail {
		return review.AnalyzerResult{
			AnalyzerName: a.Name(),
			Status:       "failed",
			Diagnostics: []string{
				"fake workspace analyzer failed",
			},
		}
	}

	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "passed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "workspace:001",
				Source:      a.Name(),
				Category:    "correctness",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Workspace analyzer finding",
				Explanation: "Test workspace analyzer finding.",
				FilePath:    "payment/retry.go",
				StartLine:   10,
				EndLine:     10,
			},
		},
	}
}

func newTestRequest() review.ReviewRequest {
	return review.ReviewRequest{
		ReviewID:            "review-001",
		InstallationID:      123,
		Organization:        "MorningBlossom",
		Repository:          "ai-code-review",
		PullRequestNumber:   7,
		BaseSHA:             "base-123",
		HeadSHA:             "head-456",
		EventType:           "pull_request",
		RequestedBy:         "test-user",
		RequestedAt:         time.Now().UTC(),
		ReviewMode:          "pull_request",
		ReviewPolicyVersion: "v1",
	}
}

func newTestOrchestrator(
	fakeGitHub *github.FakeProvider,
	analyzers []analyzer.Analyzer,
	workspaceAnalyzers []analyzer.WorkspaceAnalyzer,
	modelProvider model.ModelProvider,
	findingValidator validator.Validator,
	reviewPublisher publisher.Publisher,
) *ReviewOrchestrator {
	contextBuilder := contextbuilder.NewBuilder(
		fakeGitHub,
		20,
		200_000,
		50_000,
	)

	workspaceBuilder := workspace.NewSnapshotBuilder(
		fakeGitHub,
	)

	return NewOrchestrator(
		fakeGitHub,
		contextBuilder,
		workspaceBuilder,
		analyzers,
		workspaceAnalyzers,
		modelProvider,
		findingValidator,
		reviewPublisher,
	)
}

func TestReviewOrchestrator_Success(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	fakeAnalyzer := &analyzer.FakeAnalyzer{}

	FakeModelProvider := &contextCaptureModel{}

	fakeValidator := &validator.FakeValidator{}

	fakePublisher := &contextCapturePublisher{}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		[]analyzer.Analyzer{
			fakeAnalyzer,
		},
		nil,
		FakeModelProvider,
		fakeValidator,
		fakePublisher,
	)

	request := newTestRequest()

	result, err := orchestrator.Review(
		context.Background(),
		request,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf(
			"expected status completed, got %q",
			result.Status,
		)
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
			"expected PR number %d, got %d",
			request.PullRequestNumber,
			result.PullRequestNumber,
		)
	}

	if result.BaseSHA != request.BaseSHA {
		t.Fatalf(
			"expected base SHA %q, got %q",
			request.BaseSHA,
			result.BaseSHA,
		)
	}

	if result.HeadSHA != request.HeadSHA {
		t.Fatalf(
			"expected head SHA %q, got %q",
			request.HeadSHA,
			result.HeadSHA,
		)
	}

	if len(result.Findings) == 0 {
		t.Fatal("expected findings")
	}

	if len(result.AnalyzerSummary) != 1 {
		t.Fatalf(
			"expected 1 analyzer result, got %d",
			len(result.AnalyzerSummary),
		)
	}

	if fakePublisher.receivedResult.Status != "completed" {
		t.Fatalf(
			"expected publisher to receive completed result, got %q",
			fakePublisher.receivedResult.Status,
		)
	}
}

func TestReviewOrchestrator_IncludesAnalyzerFindings(t *testing.T) {
	fakeGitHub := &github.FakeProvider{}

	fakeAnalyzer := &analyzer.FakeAnalyzer{}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		[]analyzer.Analyzer{
			fakeAnalyzer,
		},
		nil,
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Findings) == 0 {
		t.Fatal("expected analyzer findings")
	}

	found := false

	for _, finding := range result.Findings {
		if finding.Source == fakeAnalyzer.Name() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf(
			"expected finding from analyzer %q",
			fakeAnalyzer.Name(),
		)
	}
}

func TestReviewOrchestrator_IncludesWorkspaceAnalyzerFindings(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{
		Archive: createTestRepositoryArchive(t),
	}

	workspaceAnalyzer := &fakeWorkspaceAnalyzer{}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		nil,
		[]analyzer.WorkspaceAnalyzer{
			workspaceAnalyzer,
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf(
			"expected status completed, got %q",
			result.Status,
		)
	}

	found := false

	for _, finding := range result.Findings {
		if finding.Source == workspaceAnalyzer.Name() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf(
			"expected finding from workspace analyzer %q",
			workspaceAnalyzer.Name(),
		)
	}

	foundAnalyzerResult := false

	for _, analyzerResult := range result.AnalyzerSummary {
		if analyzerResult.AnalyzerName == workspaceAnalyzer.Name() {
			foundAnalyzerResult = true
			break
		}
	}

	if !foundAnalyzerResult {
		t.Fatalf(
			"expected workspace analyzer result %q",
			workspaceAnalyzer.Name(),
		)
	}
}

func TestReviewOrchestrator_AnalyzerFailureDoesNotFailReview(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{}

	fakeAnalyzer := &analyzer.FakeAnalyzer{
		Fail: true,
	}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		[]analyzer.Analyzer{
			fakeAnalyzer,
		},
		nil,
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err != nil {
		t.Fatalf(
			"expected review to continue after analyzer failure, got %v",
			err,
		)
	}

	if result.Status != "completed" {
		t.Fatalf(
			"expected status completed, got %q",
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
}

func TestReviewOrchestrator_WorkspaceAnalyzerFailureDoesNotFailReview(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{
		Archive: createTestRepositoryArchive(t),
	}

	workspaceAnalyzer := &fakeWorkspaceAnalyzer{
		fail: true,
	}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		nil,
		[]analyzer.WorkspaceAnalyzer{
			workspaceAnalyzer,
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err != nil {
		t.Fatalf(
			"expected review to continue after workspace analyzer failure, got %v",
			err,
		)
	}

	if result.Status != "completed" {
		t.Fatalf(
			"expected status completed, got %q",
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
			"expected workspace analyzer status failed, got %q",
			result.AnalyzerSummary[0].Status,
		)
	}
}

func TestReviewOrchestrator_GitHubPullRequestError(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{
		GetPullRequestError: errors.New("GitHub unavailable"),
	}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		nil,
		nil,
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}

	if len(result.Errors) != 1 {
		t.Fatalf(
			"expected 1 error, got %d",
			len(result.Errors),
		)
	}
}

func TestReviewOrchestrator_ChangedFilesError(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{
		GetChangedFilesError: errors.New(
			"changed files unavailable",
		),
	}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		nil,
		nil,
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}

	if len(result.Errors) != 1 {
		t.Fatalf(
			"expected 1 error, got %d",
			len(result.Errors),
		)
	}
}

func TestReviewOrchestrator_WorkspaceBuildError(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{
		DownloadRepositoryArchiveError: errors.New(
			"archive unavailable",
		),
	}

	workspaceAnalyzer := &fakeWorkspaceAnalyzer{}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		nil,
		[]analyzer.WorkspaceAnalyzer{
			workspaceAnalyzer,
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}

	if len(result.Errors) != 1 {
		t.Fatalf(
			"expected 1 error, got %d",
			len(result.Errors),
		)
	}
}

func TestReviewOrchestrator_ModelFailure(
	t *testing.T,
) {
	FakeModelProvider := &model.FakeModelProvider{
		Fail: true,
	}

	orchestrator := newTestOrchestrator(
		&github.FakeProvider{},
		nil,
		nil,
		FakeModelProvider,
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

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

func TestReviewOrchestrator_ValidatorFailure(
	t *testing.T,
) {
	fakeValidator := &validator.FakeValidator{
		Fail: true,
	}

	orchestrator := newTestOrchestrator(
		&github.FakeProvider{},
		nil,
		nil,
		&model.FakeModelProvider{},
		fakeValidator,
		&publisher.FakePublisher{},
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

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

func TestReviewOrchestrator_PublisherFailure(
	t *testing.T,
) {
	fakePublisher := &publisher.FakePublisher{
		Fail: true,
	}

	orchestrator := newTestOrchestrator(
		&github.FakeProvider{},
		nil,
		nil,
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		fakePublisher,
	)

	result, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

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

func TestReviewOrchestrator_ContextBuilderIntegration(
	t *testing.T,
) {
	fakeGitHub := &github.FakeProvider{}

	FakeModelProvider := &contextCaptureModel{}

	orchestrator := newTestOrchestrator(
		fakeGitHub,
		nil,
		nil,
		FakeModelProvider,
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)

	_, err := orchestrator.Review(
		context.Background(),
		newTestRequest(),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if FakeModelProvider.receivedContext.PullRequestTitle !=
		"Add payment retry handling" {
		t.Fatalf(
			"unexpected PR title: %q",
			FakeModelProvider.receivedContext.PullRequestTitle,
		)
	}

	if FakeModelProvider.receivedContext.PullRequestBody !=
		"This PR adds retry handling for failed payments." {
		t.Fatalf(
			"unexpected PR body: %q",
			FakeModelProvider.receivedContext.PullRequestBody,
		)
	}

	if FakeModelProvider.receivedContext.HeadSHA != "head-456" {
		t.Fatalf(
			"unexpected head SHA: %q",
			FakeModelProvider.receivedContext.HeadSHA,
		)
	}

	if FakeModelProvider.receivedContext.BaseSHA != "base-123" {
		t.Fatalf(
			"unexpected base SHA: %q",
			FakeModelProvider.receivedContext.BaseSHA,
		)
	}

	if len(FakeModelProvider.receivedContext.SourceFiles) != 1 {
		t.Fatalf(
			"expected 1 source file, got %d",
			len(FakeModelProvider.receivedContext.SourceFiles),
		)
	}
}

func createTestRepositoryArchive(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer

	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	files := map[string]string{
		"repo-head/go.mod":           "module example.com/test\n",
		"repo-head/payment/retry.go": "package payment\n",
	}

	for path, content := range files {
		err := tarWriter.WriteHeader(&tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(content)),
		})
		if err != nil {
			t.Fatalf("write tar header: %v", err)
		}

		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buffer.Bytes()
}
