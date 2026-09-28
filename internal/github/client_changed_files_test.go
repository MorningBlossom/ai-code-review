package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetChangedFiles(t *testing.T) {
	auth := &fakeAuthenticator{
		token: "test-token",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}

		expectedPath := "/repos/MorningBlossom/test-repo/pulls/42/files"

		if r.URL.Path != expectedPath {
			t.Fatalf(
				"expected path %s, got %s",
				expectedPath,
				r.URL.Path,
			)
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header")
		}

		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Fatalf("unexpected Accept header")
		}

		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Fatalf("unexpected GitHub API version")
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`[
			{
				"filename": "payment/retry.go",
				"status": "modified",
				"additions": 20,
				"deletions": 5,
				"patch": "@@ -1,5 +1,20 @@"
			},
			{
				"filename": "payment/retry_test.go",
				"status": "added",
				"additions": 30,
				"deletions": 0,
				"patch": "@@ -0,0 +1,30 @@"
			}
		]`))
	}))
	defer server.Close()

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	result, err := client.GetChangedFiles(
		context.Background(),
		123,
		"MorningBlossom",
		"test-repo",
		42,
	)
	if err != nil {
		t.Fatalf("GetChangedFiles returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 changed files, got %d", len(result))
	}

	if result[0].Path != "payment/retry.go" {
		t.Fatalf("unexpected first file: %s", result[0].Path)
	}

	if result[0].Status != "modified" {
		t.Fatalf("unexpected first file status: %s", result[0].Status)
	}

	if result[0].Additions != 20 {
		t.Fatalf("expected 20 additions, got %d", result[0].Additions)
	}

	if result[0].Deletions != 5 {
		t.Fatalf("expected 5 deletions, got %d", result[0].Deletions)
	}

	if result[0].Patch != "@@ -1,5 +1,20 @@" {
		t.Fatalf("unexpected patch: %s", result[0].Patch)
	}
}

func TestClientGetChangedFilesPagination(t *testing.T) {
	auth := &fakeAuthenticator{
		token: "test-token",
	}

	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.URL.Query().Get("per_page") != "100" {
			t.Fatalf("expected per_page=100")
		}

		switch r.URL.Query().Get("page") {
		case "1":
			files := make([]map[string]interface{}, 100)

			for i := range files {
				files[i] = map[string]interface{}{
					"filename":  fmt.Sprintf("file-%d.go", i),
					"status":    "modified",
					"additions": 1,
					"deletions": 0,
					"patch":     "+change",
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(files)

		case "2":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"filename":  "file-100.go",
					"status":    "added",
					"additions": 10,
					"deletions": 0,
					"patch":     "+new file",
				},
			})

		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
		}
	}))
	defer server.Close()

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	result, err := client.GetChangedFiles(
		context.Background(),
		123,
		"MorningBlossom",
		"test-repo",
		42,
	)
	if err != nil {
		t.Fatalf("GetChangedFiles returned error: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf(
			"expected 2 requests, got %d",
			requestCount,
		)
	}

	if len(result) != 101 {
		t.Fatalf(
			"expected 101 changed files, got %d",
			len(result),
		)
	}

	if result[100].Path != "file-100.go" {
		t.Fatalf(
			"unexpected last file: %s",
			result[100].Path,
		)
	}
}
