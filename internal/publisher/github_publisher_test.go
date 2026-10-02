package publisher

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type fakeGitHubReviewClient struct {
	review          github.PullRequestReview
	existingReviews []github.PullRequestReviewInfo
	createCalls     int
	listCalls       int
	listErr         error
}

func (f *fakeGitHubReviewClient) CreatePullRequestReview(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ int,
	reviewData github.PullRequestReview,
) error {
	f.createCalls++
	f.review = reviewData
	return nil
}

func (f *fakeGitHubReviewClient) ListPullRequestReviews(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ int,
) ([]github.PullRequestReviewInfo, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.existingReviews, nil
}

func TestGitHubPublisher_Publish(t *testing.T) {
	client := &fakeGitHubReviewClient{}
	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "review-123",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "pull_request",
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

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.review.Event != "COMMENT" {
		t.Fatalf(
			"expected COMMENT event, got %q",
			client.review.Event,
		)
	}

	if client.review.Body == "" {
		t.Fatal("expected review body")
	}

	if len(client.review.Comments) != 1 {
		t.Fatalf(
			"expected 1 comment, got %d",
			len(client.review.Comments),
		)
	}

	comment := client.review.Comments[0]

	if comment.Path != "internal/payment/service.go" {
		t.Fatalf(
			"unexpected path: %q",
			comment.Path,
		)
	}

	if comment.Line != 27 {
		t.Fatalf(
			"unexpected line: %d",
			comment.Line,
		)
	}

	if comment.Side != "RIGHT" {
		t.Fatalf(
			"unexpected side: %q",
			comment.Side,
		)
	}

	if comment.Body == "" {
		t.Fatal("expected comment body")
	}
}

func TestGitHubPublisher_Publish_RequestChangesForBlockingFinding(
	t *testing.T,
) {
	client := &fakeGitHubReviewClient{}
	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "review-456",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "pull_request",
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

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)
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

func TestGitHubPublisher_Publish_SkipsDuplicateReview(
	t *testing.T,
) {
	client := &fakeGitHubReviewClient{
		existingReviews: []github.PullRequestReviewInfo{
			{
				ID:        100,
				UserLogin: "ai-code-review-bot",
				CommitSHA: "head-123",
				Body: "<!-- ai-code-review:" +
					"MorningBlossom/example-repo/pr-42/" +
					"head-123/v1/pull_request -->" +
					"\n\n## AI Code Review",
			},
		},
	}

	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "review-new",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "pull_request",
	}

	result := review.ReviewResult{
		Status: "completed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "finding-1",
				Severity:    "medium",
				Title:       "Potential issue",
				Explanation: "Potential issue detected.",
				FilePath:    "internal/service.go",
				StartLine:   10,
			},
		},
	}

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.createCalls != 0 {
		t.Fatalf(
			"expected no GitHub review creation, got %d calls",
			client.createCalls,
		)
	}
}

func TestGitHubPublisher_Publish_ManualReviewIsNotDuplicateOfAutomaticReview(
	t *testing.T,
) {
	client := &fakeGitHubReviewClient{
		existingReviews: []github.PullRequestReviewInfo{
			{
				ID:        100,
				UserLogin: "ai-code-review-bot",
				CommitSHA: "head-123",
				Body: "<!-- ai-code-review:" +
					"MorningBlossom/example-repo/pr-42/" +
					"head-123/v1/pull_request -->" +
					"\n\n## AI Code Review",
			},
		},
	}

	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "manual-review",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "manual_full_pr",
	}

	result := review.ReviewResult{
		Status: "completed",
	}

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.createCalls != 1 {
		t.Fatalf(
			"expected manual review to create a new GitHub review, got %d calls",
			client.createCalls,
		)
	}
}

func TestGitHubPublisher_Publish_SkipsDuplicateManualReview(
	t *testing.T,
) {
	client := &fakeGitHubReviewClient{
		existingReviews: []github.PullRequestReviewInfo{
			{
				ID:        101,
				UserLogin: "ai-code-review-bot",
				CommitSHA: "head-123",
				Body: "<!-- ai-code-review:" +
					"MorningBlossom/example-repo/pr-42/" +
					"head-123/v1/manual_full_pr -->" +
					"\n\n## AI Code Review",
			},
		},
	}

	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "manual-review-2",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "manual_full_pr",
	}

	result := review.ReviewResult{
		Status: "completed",
	}

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.createCalls != 0 {
		t.Fatalf(
			"expected duplicate manual review to be skipped, got %d calls",
			client.createCalls,
		)
	}
}

func TestGitHubPublisher_Publish_FailsWhenListingReviewsFails(
	t *testing.T,
) {
	client := &fakeGitHubReviewClient{
		listErr: errors.New("GitHub API returned 403"),
	}

	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "review-789",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "pull_request",
	}

	result := review.ReviewResult{
		Status: "completed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "finding-1",
				Severity:    "medium",
				Title:       "Potential issue",
				Explanation: "Potential issue detected.",
				FilePath:    "internal/service.go",
				StartLine:   10,
			},
		},
	}

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)

	if err == nil {
		t.Fatal(
			"expected error when listing existing reviews fails",
		)
	}

	if !strings.Contains(
		err.Error(),
		"list existing GitHub reviews",
	) {
		t.Fatalf(
			"expected review listing error, got %v",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"GitHub API returned 403",
	) {
		t.Fatalf(
			"expected underlying GitHub error, got %v",
			err,
		)
	}

	if client.createCalls != 0 {
		t.Fatalf(
			"expected no GitHub review creation after list failure, got %d",
			client.createCalls,
		)
	}
}

func TestGitHubPublisher_Publish_NonInlineFindingsRemainInSummary(
	t *testing.T,
) {
	client := &fakeGitHubReviewClient{}

	publisher := NewGitHubPublisher(client)

	request := review.ReviewRequest{
		ReviewID:            "review-summary",
		InstallationID:      456,
		Organization:        "MorningBlossom",
		Repository:          "example-repo",
		PullRequestNumber:   42,
		HeadSHA:             "head-123",
		ReviewPolicyVersion: "v1",
		ReviewMode:          "pull_request",
	}

	result := review.ReviewResult{
		Status: "completed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "finding-inline",
				Severity:    "medium",
				Title:       "Inline issue",
				Explanation: "This finding can be attached to a line.",
				FilePath:    "internal/service.go",
				StartLine:   10,
			},
			{
				FindingID:   "finding-summary",
				Severity:    "high",
				Title:       "Repository-level issue",
				Explanation: "This finding cannot be attached to a specific line.",
				FilePath:    "",
				StartLine:   0,
			},
		},
	}

	err := publisher.Publish(
		context.Background(),
		request,
		result,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.createCalls != 1 {
		t.Fatalf(
			"expected 1 GitHub review creation, got %d",
			client.createCalls,
		)
	}

	if len(client.review.Comments) != 1 {
		t.Fatalf(
			"expected 1 inline comment, got %d",
			len(client.review.Comments),
		)
	}

	if client.review.Comments[0].Path != "internal/service.go" {
		t.Fatalf(
			"unexpected inline comment path: %q",
			client.review.Comments[0].Path,
		)
	}

	if !strings.Contains(
		client.review.Body,
		"Repository-level issue",
	) {
		t.Fatal(
			"expected non-inline finding to appear in review summary",
		)
	}

	if !strings.Contains(
		client.review.Body,
		"internal/service.go",
	) {
		t.Fatal(
			"expected inline finding to appear in review summary",
		)
	}
}
