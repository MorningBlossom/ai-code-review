package model

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type ModelProvider interface {
	Name() string
	Review(ctx context.Context, reviewContext review.ReviewContext) ([]review.ReviewFinding, error)
}
