package github

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetFileContent(t *testing.T) {
	auth := &fakeAuthenticator{
		token: "test-token",
	}

	expectedContent := `package payment

func retryPayment() error {
	return nil
}`

	encodedContent := base64.StdEncoding.EncodeToString(
		[]byte(expectedContent),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}

		expectedPath := "/repos/MorningBlossom/test-repo/contents/payment/retry.go"

		if r.URL.Path != expectedPath {
			t.Fatalf(
				"expected path %s, got %s",
				expectedPath,
				r.URL.Path,
			)
		}

		if r.URL.Query().Get("ref") != "head-456" {
			t.Fatalf(
				"expected ref=head-456, got %s",
				r.URL.Query().Get("ref"),
			)
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header")
		}

		w.Header().Set("Content-Type", "application/json")

		_, _ = w.Write([]byte(`{
			"content": "` + encodedContent + `",
			"encoding": "base64"
		}`))
	}))
	defer server.Close()

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	result, err := client.GetFileContent(
		context.Background(),
		123,
		"MorningBlossom",
		"test-repo",
		"head-456",
		"payment/retry.go",
	)
	if err != nil {
		t.Fatalf("GetFileContent returned error: %v", err)
	}

	if result != expectedContent {
		t.Fatalf(
			"unexpected content:\n%s",
			result,
		)
	}
}

func TestClientGetFileContentRejectsUnsupportedEncoding(t *testing.T) {
	auth := &fakeAuthenticator{
		token: "test-token",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"content": "some-content",
			"encoding": "utf-8"
		}`))
	}))
	defer server.Close()

	client := NewClient(
		server.Client(),
		auth,
		server.URL,
	)

	_, err := client.GetFileContent(
		context.Background(),
		123,
		"MorningBlossom",
		"test-repo",
		"head-456",
		"payment/retry.go",
	)

	if err == nil {
		t.Fatal("expected unsupported encoding error")
	}
}
