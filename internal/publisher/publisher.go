package publisher

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Publisher interface {
	Publish(
		ctx context.Context,
		request review.ReviewRequest,
		result review.ReviewResult,
	) error
}
