package validator

import (
	"context"
	"errors"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type FakeValidator struct {
	Fail bool
}

func (f *FakeValidator) Validate(
	ctx context.Context,
	request review.ReviewRequest,
	findings []review.ReviewFinding,
) ([]review.ReviewFinding, error) {
	if f.Fail {
		return nil, errors.New("fake validator failure")
	}

	return findings, nil
}
