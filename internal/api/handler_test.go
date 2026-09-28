package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

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

	handler := NewHandler(orchestrator)

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
	handler := NewHandler(orchestrator)

	req := httptest.NewRequest(
		http.MethodPost,
		"/reviews",
		strings.NewReader(`{"review_id":`),
	)

	req.Header.Set("Content-Type", "application/json")

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
	handler := NewHandler(orchestrator)

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
