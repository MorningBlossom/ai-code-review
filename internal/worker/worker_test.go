package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/queue"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type fakeOrchestrator struct {
	mu sync.Mutex

	calls int

	requests []review.ReviewRequest

	err error

	called chan struct{}
}

func (f *fakeOrchestrator) Review(
	ctx context.Context,
	request review.ReviewRequest,
) (review.ReviewResult, error) {
	f.mu.Lock()

	f.calls++
	f.requests = append(f.requests, request)

	f.mu.Unlock()

	if f.called != nil {
		select {
		case f.called <- struct{}{}:
		default:
		}
	}

	if f.err != nil {
		return review.ReviewResult{}, f.err
	}

	return review.ReviewResult{
		ReviewID: request.ReviewID,
		Status:   "completed",
	}, nil
}

func (f *fakeOrchestrator) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

type fakeDeliveryStore struct {
	mu sync.Mutex

	forgotten []string
}

func (f *fakeDeliveryStore) Forget(deliveryID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.forgotten = append(f.forgotten, deliveryID)
}

func (f *fakeDeliveryStore) Forgotten() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make([]string, len(f.forgotten))
	copy(result, f.forgotten)

	return result
}

func TestWorker_ProcessesQueuedReview(t *testing.T) {
	q, err := queue.New(2)
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}

	orchestrator := &fakeOrchestrator{
		called: make(chan struct{}, 1),
	}

	deliveryStore := &fakeDeliveryStore{}

	w, err := New(Config{
		Queue:         q,
		Orchestrator:  orchestrator,
		DeliveryStore: deliveryStore,
		Concurrency:   1,
		ReviewTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	request := review.ReviewRequest{
		ReviewID:          "delivery-1",
		Organization:      "MorningBlossom",
		Repository:        "ai-code-review",
		PullRequestNumber: 10,
		HeadSHA:           "abc123",
	}

	if err := q.Enqueue(context.Background(), queue.Job{
		DeliveryID: "delivery-1",
		Request:    request,
	}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	select {
	case <-orchestrator.called:
	case <-time.After(time.Second):
		t.Fatal("worker did not process queued review")
	}

	if got := orchestrator.Calls(); got != 1 {
		t.Fatalf("orchestrator calls = %d, want 1", got)
	}

	if forgotten := deliveryStore.Forgotten(); len(forgotten) != 0 {
		t.Fatalf(
			"delivery store forgotten = %v, want empty",
			forgotten,
		)
	}

	q.Close()
	cancel()
	w.Wait()
}

func TestWorker_ForgetsDeliveryOnFailure(t *testing.T) {
	q, err := queue.New(2)
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}

	orchestrator := &fakeOrchestrator{
		err:    errors.New("review failed"),
		called: make(chan struct{}, 1),
	}

	deliveryStore := &fakeDeliveryStore{}

	w, err := New(Config{
		Queue:         q,
		Orchestrator:  orchestrator,
		DeliveryStore: deliveryStore,
		Concurrency:   1,
		ReviewTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	if err := q.Enqueue(context.Background(), queue.Job{
		DeliveryID: "delivery-failed",
		Request: review.ReviewRequest{
			ReviewID:          "delivery-failed",
			Organization:      "MorningBlossom",
			Repository:        "ai-code-review",
			PullRequestNumber: 11,
			HeadSHA:           "def456",
		},
	}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	select {
	case <-orchestrator.called:
	case <-time.After(time.Second):
		t.Fatal("worker did not process queued review")
	}

	deadline := time.Now().Add(time.Second)

	for time.Now().Before(deadline) {
		forgotten := deliveryStore.Forgotten()

		if len(forgotten) == 1 &&
			forgotten[0] == "delivery-failed" {
			q.Close()
			cancel()
			w.Wait()

			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf(
		"delivery was not forgotten: %v",
		deliveryStore.Forgotten(),
	)
}

func TestWorker_ProcessesMultipleJobs(t *testing.T) {
	q, err := queue.New(4)
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}

	orchestrator := &fakeOrchestrator{
		called: make(chan struct{}, 4),
	}

	deliveryStore := &fakeDeliveryStore{}

	w, err := New(Config{
		Queue:         q,
		Orchestrator:  orchestrator,
		DeliveryStore: deliveryStore,
		Concurrency:   1,
		ReviewTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	for i := 0; i < 3; i++ {
		if err := q.Enqueue(context.Background(), queue.Job{
			DeliveryID: "delivery-" + string(rune('1'+i)),
			Request: review.ReviewRequest{
				ReviewID:          "review-" + string(rune('1'+i)),
				Organization:      "MorningBlossom",
				Repository:        "ai-code-review",
				PullRequestNumber: i + 1,
				HeadSHA:           "sha",
			},
		}); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}

	deadline := time.Now().Add(time.Second)

	for time.Now().Before(deadline) {
		if orchestrator.Calls() == 3 {
			q.Close()
			cancel()
			w.Wait()

			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf(
		"orchestrator calls = %d, want 3",
		orchestrator.Calls(),
	)
}
