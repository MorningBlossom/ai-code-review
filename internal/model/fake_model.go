package model

import (
	"context"
	"errors"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type FakeModelProvider struct {
	Fail bool
}

func (f *FakeModelProvider) Name() string {
	return "fake-model"
}

func (f *FakeModelProvider) Review(
	ctx context.Context,
	reviewContext review.ReviewContext,
) ([]review.ReviewFinding, error) {
	if f.Fail {
		return nil, errors.New("fake model failure")
	}

	return []review.ReviewFinding{
		{
			FindingID:   "model-001",
			Source:      "fake-model",
			Category:    "performance",
			Severity:    "low",
			Confidence:  0.88,
			Title:       "Retry operation may need a backoff",
			Explanation: "Repeated retries without a delay can increase load on the downstream service.",
			Suggestion:  "Consider using exponential backoff between retry attempts.",
			FilePath:    "payment/retry.go",
			StartLine:   3,
			EndLine:     5,
		},
	}, nil
}
