package queue

import (
	"context"
	"errors"
	"sync"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

var (
	ErrQueueFull   = errors.New("review queue is full")
	ErrQueueClosed = errors.New("review queue is closed")
)

type Job struct {
	DeliveryID string
	Request    review.ReviewRequest
}

type Queue struct {
	jobs chan Job

	closeOnce sync.Once
}

func New(capacity int) (*Queue, error) {
	if capacity <= 0 {
		return nil, errors.New("queue capacity must be greater than 0")
	}

	return &Queue{
		jobs: make(chan Job, capacity),
	}, nil
}

func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	select {
	case q.jobs <- job:
		return nil

	default:
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return ErrQueueFull
		}
	}
}

func (q *Queue) Jobs() <-chan Job {
	return q.jobs
}

func (q *Queue) Close() {
	q.closeOnce.Do(func() {
		close(q.jobs)
	})
}

func (q *Queue) Len() int {
	return len(q.jobs)
}

func (q *Queue) Capacity() int {
	return cap(q.jobs)
}
