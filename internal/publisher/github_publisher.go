package publisher

import (
	"context"
	"fmt"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const reviewMarkerPrefix = "<!-- ai-code-review:"

type GitHubReviewClient interface {
	CreatePullRequestReview(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		pullRequestNumber int,
		review github.PullRequestReview,
	) error

	ListPullRequestReviews(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		pullRequestNumber int,
	) ([]github.PullRequestReviewInfo, error)
}

type GitHubPublisher struct {
	client GitHubReviewClient
}

func NewGitHubPublisher(client GitHubReviewClient) *GitHubPublisher {
	return &GitHubPublisher{
		client: client,
	}
}

func (p *GitHubPublisher) Publish(
	ctx context.Context,
	request review.ReviewRequest,
	result review.ReviewResult,
) error {
	if p.client == nil {
		return fmt.Errorf("GitHub review client is nil")
	}

	marker := buildReviewMarker(request)

	existingReviews, err := p.client.ListPullRequestReviews(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		return fmt.Errorf("list existing GitHub reviews: %w", err)
	}

	if hasPublishedReview(existingReviews, marker, request.HeadSHA) {
		return nil
	}

	var comments []github.PullRequestReviewComment

	for _, finding := range result.Findings {
		if finding.FilePath == "" || finding.StartLine <= 0 {
			continue
		}

		comments = append(
			comments,
			github.PullRequestReviewComment{
				Path: finding.FilePath,
				Line: finding.StartLine,
				Side: "RIGHT",
				Body: formatFindingComment(finding),
			},
		)
	}

	event := "COMMENT"

	if hasBlockingFinding(result.Findings) {
		event = "REQUEST_CHANGES"
	}

	return p.client.CreatePullRequestReview(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
		github.PullRequestReview{
			Body:     buildReviewBody(request, result),
			Event:    event,
			Comments: comments,
		},
	)
}

func buildReviewMarker(request review.ReviewRequest) string {
	mode := request.ReviewMode

	if mode == "" {
		mode = "pull_request"
	}

	return fmt.Sprintf(
		"%s%s/%s/pr-%d/%s/%s/%s -->",
		reviewMarkerPrefix,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
		request.HeadSHA,
		request.ReviewPolicyVersion,
		mode,
	)
}

func containsReviewMarker(body string, marker string) bool {
	return strings.Contains(body, marker)
}

func hasPublishedReview(
	reviews []github.PullRequestReviewInfo,
	marker string,
	headSHA string,
) bool {
	for _, existingReview := range reviews {
		if existingReview.CommitSHA != headSHA {
			continue
		}

		if containsReviewMarker(existingReview.Body, marker) {
			return true
		}
	}

	return false
}

func buildReviewBody(
	request review.ReviewRequest,
	result review.ReviewResult,
) string {
	var builder strings.Builder

	builder.WriteString(buildReviewMarker(request))
	builder.WriteString("\n\n")
	builder.WriteString("## AI Code Review\n\n")

	if len(result.Findings) == 0 {
		builder.WriteString("No actionable findings were detected.\n")
		return builder.String()
	}

	builder.WriteString(
		fmt.Sprintf(
			"Found **%d actionable finding(s)**.\n\n",
			len(result.Findings),
		),
	)

	for _, finding := range result.Findings {
		builder.WriteString(
			fmt.Sprintf(
				"- **%s** — `%s` — %s\n",
				strings.ToUpper(finding.Severity),
				finding.Title,
				finding.FilePath,
			),
		)
	}

	return builder.String()
}

func formatFindingComment(finding review.ReviewFinding) string {
	var builder strings.Builder

	builder.WriteString(
		fmt.Sprintf(
			"**%s** `%s`\n\n",
			strings.ToUpper(finding.Severity),
			finding.Title,
		),
	)

	builder.WriteString(finding.Explanation)

	if finding.Evidence != "" {
		builder.WriteString("\n\n**Evidence:**\n")
		builder.WriteString(finding.Evidence)
	}

	if finding.Suggestion != "" {
		builder.WriteString("\n\n**Suggestion:**\n")
		builder.WriteString(finding.Suggestion)
	}

	return builder.String()
}

func hasBlockingFinding(findings []review.ReviewFinding) bool {
	for _, finding := range findings {
		switch strings.ToLower(finding.Severity) {
		case "critical", "high":
			return true
		}
	}

	return false
}
