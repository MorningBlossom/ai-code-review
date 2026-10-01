package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAuthenticator struct {
	token string
	err   error
}

func (f *fakeAuthenticator) GetInstallationToken(
	_ context.Context,
	_ int64,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	return f.token, nil
}

func TestClient_GetPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", r.Method)
			}

			expectedPath := "/repos/MorningBlossom/test-repo/pulls/42"

			if r.URL.Path != expectedPath {
				t.Fatalf(
					"expected path %s, got %s",
					expectedPath,
					r.URL.Path,
				)
			}

			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatal("expected installation token")
			}

			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Fatal("expected GitHub Accept header")
			}

			if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
				t.Fatal("expected GitHub API version header")
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
					"number": 42,
					"title": "Add payment retry handling",
					"body": "Add retry handling for failed payments.",
					"user": {
						"login": "test-user"
					},
					"base": {
						"sha": "base-123"
					},
					"head": {
						"sha": "head-456"
					}
			}`))
		},
	))

	defer server.Close()

	auth := &fakeAuthenticator{
		token: "test-token",
	}

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	result, err := client.GetPullRequest(
		context.Background(),
		123,
		"MorningBlossom",
		"test-repo",
		42,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Number != 42 {
		t.Fatalf("expected number 42, got %d", result.Number)
	}

	if result.Title != "Add payment retry handling" {
		t.Fatalf(
			"expected title Add payment retry handling, got %q",
			result.Title,
		)
	}

	if result.BaseSHA != "base-123" {
		t.Fatalf(
			"expected base SHA base-123, got %q",
			result.BaseSHA,
		)
	}

	if result.HeadSHA != "head-456" {
		t.Fatalf(
			"expected head SHA head-456, got %q",
			result.HeadSHA,
		)
	}

	if result.Author != "test-user" {
		t.Fatalf(
			"expected author test-user, got %q",
			result.Author,
		)
	}
}

func TestClient_CreatePullRequestReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}

			expectedPath :=
				"/repos/MorningBlossom/example-repo/pulls/42/reviews"

			if r.URL.Path != expectedPath {
				t.Fatalf(
					"expected path %s, got %s",
					expectedPath,
					r.URL.Path,
				)
			}

			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatal("expected installation token")
			}

			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Fatal("expected GitHub Accept header")
			}

			if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
				t.Fatal("expected GitHub API version header")
			}

			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatal("expected JSON Content-Type")
			}

			var payload struct {
				Body     string `json:"body"`
				Event    string `json:"event"`
				Comments []struct {
					Path string `json:"path"`
					Line int    `json:"line"`
					Side string `json:"side"`
					Body string `json:"body"`
				} `json:"comments"`
			}

			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}

			if payload.Body != "AI review completed." {
				t.Fatalf(
					"expected body %q, got %q",
					"AI review completed.",
					payload.Body,
				)
			}

			if payload.Event != "COMMENT" {
				t.Fatalf(
					"expected event COMMENT, got %q",
					payload.Event,
				)
			}

			if len(payload.Comments) != 1 {
				t.Fatalf(
					"expected 1 comment, got %d",
					len(payload.Comments),
				)
			}

			comment := payload.Comments[0]

			if comment.Path != "internal/payment/service.go" {
				t.Fatalf(
					"unexpected comment path: %q",
					comment.Path,
				)
			}

			if comment.Line != 27 {
				t.Fatalf(
					"expected comment line 27, got %d",
					comment.Line,
				)
			}

			if comment.Side != "RIGHT" {
				t.Fatalf(
					"expected comment side RIGHT, got %q",
					comment.Side,
				)
			}

			if comment.Body != "Potential error handling issue." {
				t.Fatalf(
					"unexpected comment body: %q",
					comment.Body,
				)
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":12345}`))
		},
	))

	defer server.Close()

	auth := &fakeAuthenticator{
		token: "test-token",
	}

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	err := client.CreatePullRequestReview(
		context.Background(),
		123,
		"MorningBlossom",
		"example-repo",
		42,
		PullRequestReview{
			Body:  "AI review completed.",
			Event: "COMMENT",
			Comments: []PullRequestReviewComment{
				{
					Path: "internal/payment/service.go",
					Line: 27,
					Side: "RIGHT",
					Body: "Potential error handling issue.",
				},
			},
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_CreatePullRequestReview_AuthenticationFailure(t *testing.T) {
	auth := &fakeAuthenticator{
		err: errors.New("authentication failed"),
	}

	client := NewClient(
		http.DefaultClient,
		auth,
		"https://api.github.com",
	)

	err := client.CreatePullRequestReview(
		context.Background(),
		123,
		"MorningBlossom",
		"example-repo",
		42,
		PullRequestReview{
			Body:  "review",
			Event: "COMMENT",
		},
	)

	if err == nil {
		t.Fatal("expected authentication error")
	}

	if !strings.Contains(err.Error(), "get installation token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_CreatePullRequestReview_GitHubError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)

			_, _ = w.Write([]byte(
				`{"message":"Line could not be resolved"}`,
			))
		},
	))

	defer server.Close()

	auth := &fakeAuthenticator{
		token: "test-token",
	}

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	err := client.CreatePullRequestReview(
		context.Background(),
		123,
		"MorningBlossom",
		"example-repo",
		42,
		PullRequestReview{
			Body:  "review",
			Event: "COMMENT",
		},
	)

	if err == nil {
		t.Fatal("expected GitHub API error")
	}

	if !strings.Contains(err.Error(), "422") {
		t.Fatalf(
			"expected status 422 in error, got %v",
			err,
		)
	}

	if !strings.Contains(err.Error(), "Line could not be resolved") {
		t.Fatalf(
			"expected GitHub error message, got %v",
			err,
		)
	}
}
