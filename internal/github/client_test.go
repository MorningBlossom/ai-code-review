package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

type fakeHTTPClient struct {
	response  *http.Response
	responses []*http.Response
	request   *http.Request
	callCount int
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.request = req
	f.callCount++

	if len(f.responses) > 0 {
		index := f.callCount - 1

		if index >= len(f.responses) {
			index = len(f.responses) - 1
		}

		return f.responses[index], nil
	}

	if f.response == nil {
		return nil, errors.New("fake HTTP client has no response configured")
	}

	return f.response, nil
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

func TestClient_ListPullRequestReviews(t *testing.T) {
	authenticator := &fakeAuthenticator{
		token: "test-token",
	}

	httpClient := &fakeHTTPClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(
				strings.NewReader(`[
					{
						"id": 101,
						"user": {
							"login": "ai-code-review-bot"
						},
						"body": "## AI Code Review\n\nFound 2 actionable findings.",
						"commit_id": "abc123"
					},
					{
						"id": 102,
						"user": {
							"login": "developer"
						},
						"body": "Looks good.",
						"commit_id": "def456"
					}
				]`),
			),
		},
	}

	client := NewClient(
		httpClient,
		authenticator,
		"https://api.github.com",
	)

	reviews, err := client.ListPullRequestReviews(
		context.Background(),
		456,
		"MorningBlossom",
		"example-repo",
		42,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews))
	}

	if reviews[0].ID != 101 {
		t.Fatalf("expected first review ID 101, got %d", reviews[0].ID)
	}

	if reviews[0].UserLogin != "ai-code-review-bot" {
		t.Fatalf(
			"expected first review user ai-code-review-bot, got %q",
			reviews[0].UserLogin,
		)
	}

	if reviews[0].Body != "## AI Code Review\n\nFound 2 actionable findings." {
		t.Fatalf("unexpected first review body: %q", reviews[0].Body)
	}

	if reviews[0].CommitSHA != "abc123" {
		t.Fatalf(
			"expected first review commit abc123, got %q",
			reviews[0].CommitSHA,
		)
	}

	if reviews[1].ID != 102 {
		t.Fatalf("expected second review ID 102, got %d", reviews[1].ID)
	}

	if reviews[1].UserLogin != "developer" {
		t.Fatalf(
			"expected second review user developer, got %q",
			reviews[1].UserLogin,
		)
	}

	if reviews[1].CommitSHA != "def456" {
		t.Fatalf(
			"expected second review commit def456, got %q",
			reviews[1].CommitSHA,
		)
	}

	req := httpClient.request

	if req == nil {
		t.Fatal("expected HTTP request")
	}

	expectedURL := "https://api.github.com/repos/MorningBlossom/example-repo/pulls/42/reviews?per_page=100&page=1"

	if req.URL.String() != expectedURL {
		t.Fatalf(
			"unexpected request URL: %q",
			req.URL.String(),
		)
	}

	if req.Method != http.MethodGet {
		t.Fatalf(
			"expected GET request, got %s",
			req.Method,
		)
	}

	if req.Header.Get("Authorization") != "Bearer test-token" {
		t.Fatal("unexpected Authorization header")
	}

	if req.Header.Get("Accept") != "application/vnd.github+json" {
		t.Fatal("unexpected Accept header")
	}

	if req.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
		t.Fatal("unexpected GitHub API version")
	}
}

func TestClient_ListPullRequestReviews_AuthenticationFailure(t *testing.T) {
	authenticator := &fakeAuthenticator{
		err: errors.New("authentication failed"),
	}

	client := NewClient(
		&fakeHTTPClient{},
		authenticator,
		"https://api.github.com",
	)

	_, err := client.ListPullRequestReviews(
		context.Background(),
		456,
		"MorningBlossom",
		"example-repo",
		42,
	)
	if err == nil {
		t.Fatal("expected authentication error")
	}

	if !strings.Contains(err.Error(), "get installation token") {
		t.Fatalf(
			"expected installation token error, got %v",
			err,
		)
	}
}

func TestClient_ListPullRequestReviews_GitHubError(t *testing.T) {
	authenticator := &fakeAuthenticator{
		token: "test-token",
	}

	httpClient := &fakeHTTPClient{
		response: &http.Response{
			StatusCode: http.StatusForbidden,
			Body: io.NopCloser(
				strings.NewReader(
					`{"message":"Resource not accessible by integration"}`,
				),
			),
		},
	}

	client := NewClient(
		httpClient,
		authenticator,
		"https://api.github.com",
	)

	_, err := client.ListPullRequestReviews(
		context.Background(),
		456,
		"MorningBlossom",
		"example-repo",
		42,
	)
	if err == nil {
		t.Fatal("expected GitHub error")
	}

	if !strings.Contains(err.Error(), "403") {
		t.Fatalf(
			"expected status 403 in error, got %v",
			err,
		)
	}

	if !strings.Contains(err.Error(), "Resource not accessible by integration") {
		t.Fatalf(
			"expected GitHub error message, got %v",
			err,
		)
	}
}
