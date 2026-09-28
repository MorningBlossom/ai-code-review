package analyzer

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Analyzer interface {
	Name() string
	Analyze(ctx context.Context, reviewContext review.ReviewContext) review.AnalyzerResult
}
