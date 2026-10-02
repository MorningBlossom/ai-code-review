package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const testWebhookSecret = "test-secret"

type webhookTestOrchestrator struct {
	calls   int
	request review.ReviewRequest
	result  review.ReviewResult
	err     error
}

var _ orchestrator.Orchestrator = (*webhookTestOrchestrator)(nil)

func (f *webhookTestOrchestrator) Review(
	_ context.Context,
	request review.ReviewRequest,
) (review.ReviewResult, error) {
	f.calls++
	f.request = request

	if f.err != nil {
		return review.ReviewResult{}, f.err
	}

	if f.result.ReviewID == "" {
		f.result.ReviewID = request.ReviewID
	}

	if f.result.Status == "" {
		f.result.Status = "completed"
	}

	return f.result, nil
}

type webhookTestGitHubProvider struct {
	pullRequest github.PullRequest
	err         error

	getPullRequestCalls int
}

var _ github.Provider = (*webhookTestGitHubProvider)(nil)

func (f *webhookTestGitHubProvider) GetPullRequest(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ int,
) (github.PullRequest, error) {
	f.getPullRequestCalls++

	if f.err != nil {
		return github.PullRequest{}, f.err
	}

	return f.pullRequest, nil
}

func (f *webhookTestGitHubProvider) GetChangedFiles(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ int,
) ([]review.ChangedFile, error) {
	return nil, nil
}

func (f *webhookTestGitHubProvider) GetFileContent(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ string,
	_ string,
) (string, error) {
	return "", nil
}

func (f *webhookTestGitHubProvider) DownloadRepositoryArchive(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ string,
) ([]byte, error) {
	return nil, nil
}

type failingWebhookTestOrchestrator struct {
	calls int
	err   error
}

var _ orchestrator.Orchestrator = (*failingWebhookTestOrchestrator)(nil)

func (f *failingWebhookTestOrchestrator) Review(
	_ context.Context,
	_ review.ReviewRequest,
) (review.ReviewResult, error) {
	f.calls++

	return review.ReviewResult{}, errors.New("orchestrator failure")
}

func createWebhookRequest(
	method string,
	payload string,
	eventType string,
	deliveryID string,
) *http.Request {
	req := httptest.NewRequest(
		method,
		"/github/webhook",
		strings.NewReader(payload),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		"X-GitHub-Event",
		eventType,
	)

	req.Header.Set(
		"X-GitHub-Delivery",
		deliveryID,
	)

	req.Header.Set(
		"X-Hub-Signature-256",
		signPayload(payload, testWebhookSecret),
	)

	return req
}

func TestGitHubWebhookHandler_ReadyPROpenedTriggersReview(t *testing.T) {
	payload := `{
"action": "opened",
"installation": {
"id": 123
},
"repository": {
"name": "test-repo",
"owner": {
"login": "MorningBlossom"
}
},
"pull_request": {
"number": 42,
"title": "Add payment retry",
"body": "Adds retry handling",
"draft": false,
"user": {
"login": "test-user"
},
"base": {
"sha": "base-123"
},
"head": {
"sha": "head-456"
}
}
}`

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-opened-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			orchestrator.calls,
		)
	}

	if orchestrator.request.ReviewMode != "pull_request" {
		t.Fatalf(
			"expected review mode pull_request, got %q",
			orchestrator.request.ReviewMode,
		)
	}

	if orchestrator.request.HeadSHA != "head-456" {
		t.Fatalf(
			"expected head SHA head-456, got %q",
			orchestrator.request.HeadSHA,
		)
	}

	if orchestrator.request.BaseSHA != "base-123" {
		t.Fatalf(
			"expected base SHA base-123, got %q",
			orchestrator.request.BaseSHA,
		)
	}
}

func TestGitHubWebhookHandler_DraftPROpenedDoesNotTriggerReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"opened",
		true,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-draft-opened",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_DraftPRSynchronizedDoesNotTriggerReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"synchronize",
		true,
		"base-123",
		"head-new",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-draft-sync",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_ReadyForReviewTriggersReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"ready_for_review",
		false,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-ready-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			orchestrator.calls,
		)
	}

	if orchestrator.request.ReviewMode != "pull_request" {
		t.Fatalf(
			"expected review mode pull_request, got %q",
			orchestrator.request.ReviewMode,
		)
	}
}

func TestGitHubWebhookHandler_ReopenedReadyPRTriggersReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"reopened",
		false,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-reopened-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_ReopenedDraftPRDoesNotTriggerReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"reopened",
		true,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-reopened-draft",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_SynchronizeDoesNotTriggerReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"synchronize",
		false,
		"base-123",
		"head-new",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-sync-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected synchronize to not trigger review, got %d calls",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_MBCommandTriggersFullPRReview(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"@mb-ai",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: github.PullRequest{
			Number:  42,
			Title:   "Add payment retry",
			Body:    "Adds retry handling",
			BaseSHA: "base-current",
			HeadSHA: "head-current",
			Author:  "test-user",
			Draft:   false,
		},
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-command-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			orchestrator.calls,
		)
	}

	if githubProvider.getPullRequestCalls != 1 {
		t.Fatalf(
			"expected current PR to be fetched once, got %d",
			githubProvider.getPullRequestCalls,
		)
	}

	if orchestrator.request.ReviewMode != "manual_full_pr" {
		t.Fatalf(
			"expected review mode manual_full_pr, got %q",
			orchestrator.request.ReviewMode,
		)
	}

	if orchestrator.request.HeadSHA != "head-current" {
		t.Fatalf(
			"expected current head SHA head-current, got %q",
			orchestrator.request.HeadSHA,
		)
	}

	if orchestrator.request.BaseSHA != "base-current" {
		t.Fatalf(
			"expected current base SHA base-current, got %q",
			orchestrator.request.BaseSHA,
		)
	}
}

func TestGitHubWebhookHandler_MBCommandIsCaseInsensitive(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"@MB-AI",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: readyPullRequest(),
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-command-case",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_MBCommandTrimsWhitespace(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"  \n @mb-ai \t\n ",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: readyPullRequest(),
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-command-whitespace",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_OtherCommentDoesNotTriggerReview(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"Please review this PR",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: readyPullRequest(),
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-comment-normal",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}

	if githubProvider.getPullRequestCalls != 0 {
		t.Fatalf(
			"expected GitHub PR not to be fetched, got %d calls",
			githubProvider.getPullRequestCalls,
		)
	}
}

func TestGitHubWebhookHandler_MBCommandOnDraftPRDoesNotTriggerReview(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"@mb-ai",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: github.PullRequest{
			Number:  42,
			BaseSHA: "base-123",
			HeadSHA: "head-456",
			Draft:   true,
		},
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-command-draft",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if githubProvider.getPullRequestCalls != 1 {
		t.Fatalf(
			"expected current PR to be fetched once, got %d",
			githubProvider.getPullRequestCalls,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls for draft PR, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_NonPRIssueCommentDoesNotTriggerReview(
	t *testing.T,
) {
	payload := `{
"action": "created",
"installation": {
"id": 123
},
"repository": {
"name": "test-repo",
"owner": {
"login": "MorningBlossom"
}
},
"issue": {
"number": 42
},
"comment": {
"body": "@mb-ai",
"user": {
"login": "test-user"
}
}
}`

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: readyPullRequest(),
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-normal-issue",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}

	if githubProvider.getPullRequestCalls != 0 {
		t.Fatalf(
			"expected no GitHub PR lookup, got %d calls",
			githubProvider.getPullRequestCalls,
		)
	}
}

func TestGitHubWebhookHandler_MBCommandUsesLatestPullRequest(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"@mb-ai",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		pullRequest: github.PullRequest{
			Number:  42,
			Title:   "Latest title",
			BaseSHA: "latest-base",
			HeadSHA: "latest-head",
			Draft:   false,
		},
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-latest-pr",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.request.BaseSHA != "latest-base" {
		t.Fatalf(
			"expected latest base SHA, got %q",
			orchestrator.request.BaseSHA,
		)
	}

	if orchestrator.request.HeadSHA != "latest-head" {
		t.Fatalf(
			"expected latest head SHA, got %q",
			orchestrator.request.HeadSHA,
		)
	}
}

func TestGitHubWebhookHandler_DuplicateDeliveryDoesNotTriggerSecondReview(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"opened",
		false,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req1 := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-duplicate-001",
	)

	rec1 := httptest.NewRecorder()

	handler.HandleWebhook(rec1, req1)

	if rec1.Code != http.StatusAccepted {
		t.Fatalf(
			"first request: expected 202, got %d",
			rec1.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected first delivery to call orchestrator once, got %d",
			orchestrator.calls,
		)
	}

	req2 := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-duplicate-001",
	)

	rec2 := httptest.NewRecorder()

	handler.HandleWebhook(rec2, req2)

	if rec2.Code != http.StatusAccepted {
		t.Fatalf(
			"duplicate request: expected 202, got %d",
			rec2.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected duplicate delivery to be ignored, got %d calls",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_FailedReviewCanBeRetried(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"opened",
		false,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{
		err: errors.New("orchestrator failure"),
	}

	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req1 := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-retry-001",
	)

	rec1 := httptest.NewRecorder()

	handler.HandleWebhook(rec1, req1)

	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf(
			"first request: expected 500, got %d",
			rec1.Code,
		)
	}

	if orchestrator.calls != 1 {
		t.Fatalf(
			"expected first attempt to call orchestrator once, got %d",
			orchestrator.calls,
		)
	}

	req2 := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-retry-001",
	)

	rec2 := httptest.NewRecorder()

	handler.HandleWebhook(rec2, req2)

	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf(
			"retry request: expected 500, got %d",
			rec2.Code,
		)
	}

	if orchestrator.calls != 2 {
		t.Fatalf(
			"expected retry to call orchestrator again, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_MBCommandGitHubFailureAllowsRetry(
	t *testing.T,
) {
	payload := issueCommentPayload(
		"@mb-ai",
		42,
	)

	githubProvider := &webhookTestGitHubProvider{
		err: errors.New("GitHub API failure"),
	}

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		githubProvider,
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issue_comment",
		"delivery-command-github-failure",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status 500, got %d",
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected orchestrator not to be called, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_InvalidSignature(t *testing.T) {
	payload := `{
"action": "opened"
}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/github/webhook",
		strings.NewReader(payload),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		"X-GitHub-Event",
		"pull_request",
	)

	req.Header.Set(
		"X-GitHub-Delivery",
		"delivery-invalid-signature",
	)

	req.Header.Set(
		"X-Hub-Signature-256",
		"sha256=invalid",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_InvalidJSON(t *testing.T) {
	payload := `{"action":`

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-invalid-json",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			rec.Code,
		)
	}
}

func TestGitHubWebhookHandler_NonPullRequestEvent(t *testing.T) {
	payload := `{
"action": "created"
}`

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"issues",
		"delivery-issues-001",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_UnsupportedPullRequestAction(
	t *testing.T,
) {
	payload := pullRequestPayload(
		"closed",
		false,
		"base-123",
		"head-456",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-closed-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusAccepted,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/github/webhook",
		nil,
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			rec.Code,
		)
	}
}

func TestGitHubWebhookHandler_RequestBodyTooLarge(t *testing.T) {
	payload := strings.Repeat(
		"a",
		maxWebhookPayloadSize+1,
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-large-payload",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusRequestEntityTooLarge,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func TestGitHubWebhookHandler_RejectsUnauthorizedOrganization(
	t *testing.T,
) {
	payload := `{
"action": "opened",
"installation": {
"id": 12345
},
"repository": {
"name": "test-repo",
"owner": {
"login": "UnauthorizedOrg"
}
},
"pull_request": {
"number": 10,
"title": "Test PR",
"body": "Test",
"draft": false,
"user": {
"login": "test-user"
},
"base": {
"sha": "base-123"
},
"head": {
"sha": "head-123"
}
}
}`

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-unauthorized-org",
	)

	orchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		orchestrator,
		&webhookTestGitHubProvider{},
		testWebhookSecret,
		deliveryStore,
		"MorningBlossom",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			rec.Code,
		)
	}

	if orchestrator.calls != 0 {
		t.Fatalf(
			"expected no orchestrator calls, got %d",
			orchestrator.calls,
		)
	}
}

func pullRequestPayload(
	action string,
	draft bool,
	baseSHA string,
	headSHA string,
) string {
	return `{
"action": "` + action + `",
"installation": {
"id": 123
},
"repository": {
"name": "test-repo",
"owner": {
"login": "MorningBlossom"
}
},
"pull_request": {
"number": 42,
"title": "Test PR",
"body": "Test",
"draft": ` + boolString(draft) + `,
"user": {
"login": "test-user"
},
"base": {
"sha": "` + baseSHA + `"
},
"head": {
"sha": "` + headSHA + `"
}
}
}`
}

func issueCommentPayload(comment string, number int) string {
	payload := map[string]interface{}{
		"action": "created",
		"installation": map[string]interface{}{
			"id": int64(12345),
		},
		"repository": map[string]interface{}{
			"name": "test-repository",
			"owner": map[string]interface{}{
				"login": "MorningBlossom",
			},
		},
		"issue": map[string]interface{}{
			"number": number,
			"pull_request": map[string]interface{}{
				"url": "https://api.github.com/repos/MorningBlossom/test-repository/pulls/42",
			},
		},
		"comment": map[string]interface{}{
			"body": comment,
			"user": map[string]interface{}{
				"login": "test-user",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func readyPullRequest() github.PullRequest {
	return github.PullRequest{
		Number:  42,
		Title:   "Test PR",
		Body:    "Test",
		BaseSHA: "base-123",
		HeadSHA: "head-456",
		Author:  "test-user",
		Draft:   false,
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}

	return "false"
}

func signPayload(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
