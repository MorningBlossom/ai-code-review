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

	if result.Status == "completed_with_analyzer_failures" {
		builder.WriteString("⚠️ Review could not be completed because one or more analyzers failed.\n\n")

		for _, analyzerResult := range result.AnalyzerSummary {
			if analyzerResult.Status != "failed" {
				continue
			}

			_, _ = fmt.Fprintf(&builder, "### ❌ %s analyzer failed\n\n", analyzerResult.AnalyzerName)

			if len(analyzerResult.Diagnostics) > 0 {
				builder.WriteString("**Reason:**\n")
				for _, diagnostic := range analyzerResult.Diagnostics {
					_, _ = fmt.Fprintf(&builder, "- %s\n", strings.TrimSpace(diagnostic))
				}
				builder.WriteString("\n")
			}

			command, fix := analyzerRemediation(analyzerResult.AnalyzerName)
			if command != "" {
				_, _ = fmt.Fprintf(&builder, "**Run:** `%s`\n\n", command)
			}
			_, _ = fmt.Fprintf(&builder, "**Fix:** %s\n\n", fix)
		}

		builder.WriteString("After fixing the analyzer issue, push the changes and run `@mb-ai` again.\n\n")
		builder.WriteString("_The AI model review was skipped because the repository could not be analyzed reliably._\n")
		return builder.String()
	}

	if len(result.Findings) == 0 {
		builder.WriteString("No actionable findings were detected.\n")
		return builder.String()
	}

	_, _ = fmt.Fprintf(&builder, "Found **%d actionable finding(s)**.\n\n", len(result.Findings))

	for _, finding := range result.Findings {
		_, _ = fmt.Fprintf(&builder, "- **%s** — `%s` — %s\n", strings.ToUpper(finding.Severity), finding.Title, finding.FilePath)
	}

	return builder.String()
}

func analyzerRemediation(analyzerName string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(analyzerName)) {
	case "gofmt":
		return "gofmt -w .", "Format the Go source files and commit the resulting changes."
	case "go-vet", "govet", "go vet":
		return "go vet ./...", "Fix the reported go vet diagnostics and run the command again."
	case "go-test", "gotest", "go test":
		return "go test ./...", "Fix the failing tests and verify the full test suite passes."
	case "go-race", "gorace", "go test -race":
		return "go test -race ./...", "Fix any race detector failures and verify the race-enabled test suite passes."
	case "staticcheck":
		return "staticcheck ./...", "Fix the reported Staticcheck diagnostics and run the command again."
	case "gosec":
		return "gosec ./...", "Review and fix the reported security diagnostics, then rerun Gosec."
	case "govulncheck":
		return "govulncheck ./...", "Review the reported dependency or vulnerability findings and remediate them before rerunning Govulncheck."
	default:
		return "", "Review the analyzer diagnostics above and rerun the analyzer after fixing the reported issue."
	}
}
func formatFindingComment(finding review.ReviewFinding) string {
	var builder strings.Builder

	_, _ = fmt.Fprintf(
		&builder,
		"**%s** `%s`\n\n",
		strings.ToUpper(finding.Severity),
		finding.Title,
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
