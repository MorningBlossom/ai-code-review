package validator

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Validator interface {
	Validate(
		ctx context.Context,
		request review.ReviewRequest,
		findings []review.ReviewFinding,
	) ([]review.ReviewFinding, error)
}
