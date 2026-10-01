package publisher

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type fakeGitHubReviewClient struct {
	review github.PullRequestReview
}

func (f *fakeGitHubReviewClient) CreatePullRequestReview(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ int,
	reviewData github.PullRequestReview,
) error {
	f.review = reviewData
	return nil
}

func TestGitHubPublisher_Publish(t *testing.T) {
	client := &fakeGitHubReviewClient{}
	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:          "review-123",
		InstallationID:    456,
		Organization:      "MorningBlossom",
		Repository:        "example-repo",
		PullRequestNumber: 42,
	}

	result := review.ReviewResult{
		Status: "completed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "finding-1",
				Source:      "go-vet",
				Category:    "bug",
				Severity:    "medium",
				Confidence:  0.95,
				Title:       "Potential error",
				Explanation: "The returned error is ignored.",
				Suggestion:  "Handle the returned error.",
				FilePath:    "internal/payment/service.go",
				StartLine:   27,
				EndLine:     27,
				Evidence:    "err := processPayment()",
			},
		},
	}

	err := publisher.Publish(context.Background(), request, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.review.Event != "COMMENT" {
		t.Fatalf("expected COMMENT event, got %q", client.review.Event)
	}

	if client.review.Body == "" {
		t.Fatal("expected review body")
	}

	if len(client.review.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(client.review.Comments))
	}

	comment := client.review.Comments[0]

	if comment.Path != "internal/payment/service.go" {
		t.Fatalf("unexpected path: %q", comment.Path)
	}

	if comment.Line != 27 {
		t.Fatalf("unexpected line: %d", comment.Line)
	}

	if comment.Side != "RIGHT" {
		t.Fatalf("unexpected side: %q", comment.Side)
	}

	if comment.Body == "" {
		t.Fatal("expected comment body")
	}
}

func TestGitHubPublisher_Publish_RequestChangesForBlockingFinding(t *testing.T) {
	client := &fakeGitHubReviewClient{}
	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:          "review-456",
		InstallationID:    456,
		Organization:      "MorningBlossom",
		Repository:        "example-repo",
		PullRequestNumber: 42,
	}

	result := review.ReviewResult{
		Status: "completed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "finding-1",
				Source:      "security",
				Category:    "security",
				Severity:    "high",
				Confidence:  0.99,
				Title:       "TLS verification disabled",
				Explanation: "TLS certificate verification is disabled.",
				Suggestion:  "Enable certificate verification.",
				FilePath:    "internal/client/client.go",
				StartLine:   15,
				EndLine:     15,
			},
		},
	}

	err := publisher.Publish(context.Background(), request, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.review.Event != "REQUEST_CHANGES" {
		t.Fatalf(
			"expected REQUEST_CHANGES event, got %q",
			client.review.Event,
		)
	}
}
