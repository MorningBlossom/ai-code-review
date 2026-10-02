package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const testAPIToken = "test-api-token"

type fakeOrchestrator struct {
	called bool
	result review.ReviewResult
	err    error
}

func (f *fakeOrchestrator) Review(
	ctx context.Context,
	request review.ReviewRequest,
) (review.ReviewResult, error) {
	f.called = true

	if f.err != nil {
		return review.ReviewResult{}, f.err
	}

	return f.result, nil
}

func TestCreateReview_Success(t *testing.T) {
	orchestrator := &fakeOrchestrator{
		result: review.ReviewResult{
			ReviewID: "review-test-001",
			Status:   "completed",
		},
	}

	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	body := `{
"review_id": "review-test-001",
"organization": "MorningBlossom",
"repository": "test-repo",
"pull_request_number": 10,
"head_sha": "head-123"
}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIToken)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if !orchestrator.called {
		t.Fatal("expected orchestrator to be called")
	}
}

func TestCreateReview_InvalidJSON(t *testing.T) {
	orchestrator := &fakeOrchestrator{}
	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(`{"review_id":`),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIToken)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal("orchestrator should not be called for invalid JSON")
	}
}

func TestCreateReview_MissingReviewID(t *testing.T) {
	orchestrator := &fakeOrchestrator{}
	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	body := `{
"organization": "MorningBlossom",
"repository": "test-repo",
"pull_request_number": 10,
"head_sha": "head-123"
}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIToken)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal("orchestrator should not be called for invalid request")
	}
}

func TestCreateReview_MissingAuthorization(t *testing.T) {
	orchestrator := &fakeOrchestrator{}
	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	body := `{
		"review_id": "review-test-001",
		"organization": "MorningBlossom",
		"repository": "test-repo",
		"pull_request_number": 10,
		"head_sha": "head-123"
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal("orchestrator should not be called without authentication")
	}
}

func TestCreateReview_InvalidAuthorization(t *testing.T) {
	orchestrator := &fakeOrchestrator{}
	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	body := `{
		"review_id": "review-test-001",
		"organization": "MorningBlossom",
		"repository": "test-repo",
		"pull_request_number": 10,
		"head_sha": "head-123"
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal("orchestrator should not be called with invalid authentication")
	}
}

func TestCreateReview_InvalidAuthorizationScheme(t *testing.T) {
	orchestrator := &fakeOrchestrator{}
	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	body := `{
		"review_id": "review-test-001",
		"organization": "MorningBlossom",
		"repository": "test-repo",
		"pull_request_number": 10,
		"head_sha": "head-123"
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+testAPIToken)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal("orchestrator should not be called with an invalid authorization scheme")
	}
}

func TestCreateReview_RequestBodyTooLarge(t *testing.T) {
	orchestrator := &fakeOrchestrator{}
	handler := NewHandler(orchestrator, testAPIToken, "MorningBlossom")

	payload := strings.Repeat("a", maxReviewPayloadSize+1)

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(payload),
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+testAPIToken,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusRequestEntityTooLarge,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal(
			"orchestrator should not be called for oversized request",
		)
	}
}
func TestCreateReview_UnauthorizedOrganization(t *testing.T) {
	orchestrator := &fakeOrchestrator{}

	handler := NewHandler(
		orchestrator,
		testAPIToken,
		"MorningBlossom",
	)

	requestBody := review.ReviewRequest{
		ReviewID:          "review-unauthorized-org",
		Organization:      "otherORGANISATION",
		Repository:        "test-repository",
		PullRequestNumber: 1,
		HeadSHA:           "abc123",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		bytes.NewReader(body),
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+testAPIToken,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}

	if orchestrator.called {
		t.Fatal(
			"orchestrator should not be called for unauthorized organization",
		)
	}
}
func TestCreateReview_InternalErrorDoesNotLeakDetails(t *testing.T) {
	orchestrator := &failingWebhookTestOrchestrator{
		err: errors.New("sensitive internal error: database password=secret"),
	}

	handler := NewHandler(
		orchestrator,
		testAPIToken,
		"MorningBlossom",
	)

	requestBody := review.ReviewRequest{
		ReviewID:          "review-internal-error",
		Organization:      "MorningBlossom",
		Repository:        "test-repository",
		PullRequestNumber: 1,
		HeadSHA:           "abc123",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		bytes.NewReader(body),
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+testAPIToken,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.CreateReview(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			recorder.Code,
		)
	}

	responseBody := recorder.Body.String()

	if strings.Contains(
		responseBody,
		"sensitive internal error",
	) {
		t.Fatal("internal error details were exposed")
	}

	if strings.Contains(
		responseBody,
		"database password",
	) {
		t.Fatal("sensitive information was exposed")
	}

	if !strings.Contains(
		responseBody,
		"review failed",
	) {
		t.Fatal("expected generic error response")
	}
}
