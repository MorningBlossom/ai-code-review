package publisher

import (
	"context"
	"errors"
	"log"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type FakePublisher struct {
	Fail bool
}

func (f *FakePublisher) Publish(
	ctx context.Context,
	request review.ReviewRequest,
	result review.ReviewResult,
) error {
	if f.Fail {
		return errors.New("fake publisher failure")
	}

	log.Printf(
		"fake publisher: review=%s findings=%d",
		result.ReviewID,
		len(result.Findings),
	)

	return nil
}
