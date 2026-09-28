package review

import "context"

type Orchestrator interface {
	Review(
		ctx context.Context,
		request ReviewRequest,
	) (ReviewResult, error)
}
