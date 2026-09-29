package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestLocalProviderReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "{\"findings\":[{\"category\":\"security\",\"severity\":\"high\",\"confidence\":0.95,\"title\":\"Hardcoded secret\",\"explanation\":\"A credential is embedded in source code.\",\"suggestion\":\"Move the credential to a secure configuration source.\",\"file_path\":\"config.go\",\"start_line\":10,\"end_line\":10,\"evidence\":\"password := \\\"secret\\\"\"}]}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	provider := NewLocalProvider(Config{
		BaseURL:        server.URL,
		Model:          "test-model",
		TimeoutSeconds: 5,
	})

	ctx := context.Background()

	findings, err := provider.Review(ctx, review.ReviewContext{
		PullRequestTitle: "Test PR",
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "config.go",
				Status: "modified",
				Patch:  "+password := \"secret\"",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "config.go",
				Content: "package main\n\npassword := \"secret\"",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Source != "local-model" {
		t.Fatalf("expected source local-model, got %s", findings[0].Source)
	}

	if findings[0].Category != "security" {
		t.Fatalf("expected security category, got %s", findings[0].Category)
	}

	if findings[0].FilePath != "config.go" {
		t.Fatalf("expected config.go, got %s", findings[0].FilePath)
	}
}

func TestLocalProviderRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model unavailable", http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewLocalProvider(Config{
		BaseURL:        server.URL,
		Model:          "test-model",
		TimeoutSeconds: 5,
	})

	_, err := provider.Review(context.Background(), review.ReviewContext{})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLocalProviderRejectsEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	provider := NewLocalProvider(Config{
		BaseURL:        server.URL,
		Model:          "test-model",
		TimeoutSeconds: 5,
	})

	_, err := provider.Review(context.Background(), review.ReviewContext{})

	if err == nil {
		t.Fatal("expected error")
	}
}
