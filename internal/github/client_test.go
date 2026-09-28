package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeAuthenticator struct {
	token string
}

func (f *fakeAuthenticator) GetInstallationToken(
	_ context.Context,
	_ int64,
) (string, error) {
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
