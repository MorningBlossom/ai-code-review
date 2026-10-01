package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const testWebhookSecret = "test-secret"

type failingWebhookTestOrchestrator struct {
	calls int
}

func (f *failingWebhookTestOrchestrator) Review(
	_ context.Context,
	_ review.ReviewRequest,
) (review.ReviewResult, error) {
	f.calls++

	return review.ReviewResult{}, errors.New("orchestrator failure")
}

type webhookTestOrchestrator struct {
	called  bool
	request review.ReviewRequest
}

func (f *webhookTestOrchestrator) Review(
	_ context.Context,
	request review.ReviewRequest,
) (review.ReviewResult, error) {
	f.called = true
	f.request = request

	return review.ReviewResult{
		ReviewID: request.ReviewID,
		Status:   "completed",
	}, nil
}

func signPayload(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-GitHub-Delivery", deliveryID)
	req.Header.Set(
		"X-Hub-Signature-256",
		signPayload(payload, testWebhookSecret),
	)

	return req
}

func TestGitHubWebhookHandler_ValidPullRequestOpened(t *testing.T) {
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

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-001",
	)

	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestGitHubWebhookHandler_SupportedActions(t *testing.T) {
	actions := []string{
		"opened",
		"synchronize",
		"reopened",
		"ready_for_review",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			payload := `{
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

			fakeOrchestrator := &webhookTestOrchestrator{}
			deliveryStore := NewDeliveryStore()

			handler := NewGithubWebhookHandler(
				fakeOrchestrator,
				testWebhookSecret,
				deliveryStore,
			)

			req := createWebhookRequest(
				http.MethodPost,
				payload,
				"pull_request",
				"delivery-"+action,
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
		})
	}
}

func TestGitHubWebhookHandler_UnsupportedAction(t *testing.T) {
	payload := `{
		"action": "closed",
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
			"base": {
				"sha": "base-123"
			},
			"head": {
				"sha": "head-456"
			}
		}
	}`

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
	)

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-closed",
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-invalid")
	req.Header.Set(
		"X-Hub-Signature-256",
		"sha256=invalid",
	)

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
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
}

func TestGitHubWebhookHandler_InvalidJSON(t *testing.T) {
	payload := `{"action":`

	req := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-invalid-json",
	)

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
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

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
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
}

func TestGitHubWebhookHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/github/webhook",
		nil,
	)

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
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

func TestGitHubWebhookHandler_DuplicateDelivery(t *testing.T) {
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
			"title": "Test PR",
			"body": "Test",
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

	fakeOrchestrator := &webhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		fakeOrchestrator,
		testWebhookSecret,
		deliveryStore,
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
		t.Fatalf("first request: expected 202, got %d", rec1.Code)
	}

	if !fakeOrchestrator.called {
		t.Fatal("expected orchestrator to be called for first delivery")
	}

	fakeOrchestrator.called = false

	req2 := createWebhookRequest(
		http.MethodPost,
		payload,
		"pull_request",
		"delivery-duplicate-001",
	)

	rec2 := httptest.NewRecorder()

	handler.HandleWebhook(rec2, req2)

	if rec2.Code != http.StatusAccepted {
		t.Fatalf("duplicate request: expected 202, got %d", rec2.Code)
	}

	if fakeOrchestrator.called {
		t.Fatal("expected duplicate delivery not to call orchestrator")
	}
}
func TestGitHubWebhookHandler_RetriesFailedDelivery(t *testing.T) {
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
			"title": "Test PR",
			"body": "Test",
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

	failingOrchestrator := &failingWebhookTestOrchestrator{}
	deliveryStore := NewDeliveryStore()

	handler := NewGithubWebhookHandler(
		failingOrchestrator,
		testWebhookSecret,
		deliveryStore,
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

	if failingOrchestrator.calls != 1 {
		t.Fatalf(
			"expected orchestrator to be called once, got %d",
			failingOrchestrator.calls,
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

	if failingOrchestrator.calls != 2 {
		t.Fatalf(
			"expected retry to call orchestrator again, got %d calls",
			failingOrchestrator.calls,
		)
	}
}
