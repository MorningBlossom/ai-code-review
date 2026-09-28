package orchestrator

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type FakeOrchestrator struct{}

func (f *FakeOrchestrator) Review(
	ctx context.Context,
	request review.ReviewRequest,
) (review.ReviewResult, error) {
	return review.ReviewResult{
		ReviewID:          request.ReviewID,
		Organization:      request.Organization,
		Repository:        request.Repository,
		PullRequestNumber: request.PullRequestNumber,
		BaseSHA:           request.BaseSHA,
		HeadSHA:           request.HeadSHA,
		Status:            "completed",
		ModelSummary:      "Fake review completed",
	}, nil
}
