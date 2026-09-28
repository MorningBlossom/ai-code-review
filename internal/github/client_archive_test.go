package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type archiveTestAuthenticator struct {
	token string
}

func (a *archiveTestAuthenticator) GetInstallationToken(
	ctx context.Context,
	installationID int64,
) (string, error) {
	return a.token, nil
}

func TestClient_DownloadRepositoryArchive(t *testing.T) {
	expectedArchive := []byte("fake-tar-gz-archive")

	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.Method != http.MethodGet {
				t.Errorf(
					"expected GET, got %s",
					request.Method,
				)
			}

			expectedPath :=
				"/repos/MorningBlossom/ai-code-review/tarball/head-456"

			if request.URL.Path != expectedPath {
				t.Errorf(
					"expected path %q, got %q",
					expectedPath,
					request.URL.Path,
				)
			}

			if request.Header.Get("Authorization") !=
				"Bearer installation-token" {
				t.Errorf(
					"unexpected authorization header: %q",
					request.Header.Get("Authorization"),
				)
			}

			if request.Header.Get("Accept") !=
				"application/vnd.github+json" {
				t.Errorf(
					"unexpected accept header: %q",
					request.Header.Get("Accept"),
				)
			}

			if request.Header.Get("X-GitHub-Api-Version") !=
				"2022-11-28" {
				t.Errorf(
					"unexpected GitHub API version: %q",
					request.Header.Get("X-GitHub-Api-Version"),
				)
			}

			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(expectedArchive)
		}),
	)

	defer server.Close()

	client := NewClient(
		server.Client(),
		&archiveTestAuthenticator{
			token: "installation-token",
		},
		server.URL,
	)

	archive, err := client.DownloadRepositoryArchive(
		context.Background(),
		123,
		"MorningBlossom",
		"ai-code-review",
		"head-456",
	)

	if err != nil {
		t.Fatalf(
			"expected no error, got %v",
			err,
		)
	}

	if string(archive) != string(expectedArchive) {
		t.Fatalf(
			"expected archive %q, got %q",
			string(expectedArchive),
			string(archive),
		)
	}
}

func TestClient_DownloadRepositoryArchive_GitHubError(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			http.Error(
				writer,
				"not found",
				http.StatusNotFound,
			)
		}),
	)

	defer server.Close()

	client := NewClient(
		server.Client(),
		&archiveTestAuthenticator{
			token: "installation-token",
		},
		server.URL,
	)

	_, err := client.DownloadRepositoryArchive(
		context.Background(),
		123,
		"MorningBlossom",
		"ai-code-review",
		"head-456",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(
		err.Error(),
		"repository archive status 404",
	) {
		t.Fatalf(
			"expected status error, got %v",
			err,
		)
	}
}

func TestClient_DownloadRepositoryArchive_EmptyResponse(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.WriteHeader(http.StatusOK)
		}),
	)

	defer server.Close()

	client := NewClient(
		server.Client(),
		&archiveTestAuthenticator{
			token: "installation-token",
		},
		server.URL,
	)

	_, err := client.DownloadRepositoryArchive(
		context.Background(),
		123,
		"MorningBlossom",
		"ai-code-review",
		"head-456",
	)

	if err == nil {
		t.Fatal("expected error for empty archive")
	}

	if !strings.Contains(
		err.Error(),
		"empty repository archive",
	) {
		t.Fatalf(
			"expected empty archive error, got %v",
			err,
		)
	}
}
