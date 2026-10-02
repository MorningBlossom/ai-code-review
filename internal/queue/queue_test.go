package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestQueue_EnqueueAndReceive(t *testing.T) {
	q, err := New(2)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := review.ReviewRequest{
		ReviewID:          "delivery-1",
		Organization:      "MorningBlossom",
		Repository:        "ai-code-review",
		PullRequestNumber: 10,
		HeadSHA:           "abc123",
	}

	job := Job{
		DeliveryID: "delivery-1",
		Request:    request,
	}

	if err := q.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	select {
	case received := <-q.Jobs():
		if received.DeliveryID != "delivery-1" {
			t.Fatalf(
				"DeliveryID = %q, want %q",
				received.DeliveryID,
				"delivery-1",
			)
		}

		if received.Request.ReviewID != "delivery-1" {
			t.Fatalf(
				"ReviewID = %q, want %q",
				received.Request.ReviewID,
				"delivery-1",
			)
		}

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued job")
	}
}

func TestQueue_ReturnsFullWhenCapacityReached(t *testing.T) {
	q, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	job := Job{
		DeliveryID: "delivery-1",
	}

	if err := q.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("first Enqueue() error = %v", err)
	}

	err = q.Enqueue(context.Background(), Job{
		DeliveryID: "delivery-2",
	})

	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf(
			"second Enqueue() error = %v, want %v",
			err,
			ErrQueueFull,
		)
	}
}

func TestQueue_EnqueueHonorsCanceledContext(t *testing.T) {
	q, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := q.Enqueue(context.Background(), Job{
		DeliveryID: "delivery-1",
	}); err != nil {
		t.Fatalf("first Enqueue() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = q.Enqueue(ctx, Job{
		DeliveryID: "delivery-2",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Enqueue() error = %v, want context.Canceled",
			err,
		)
	}
}

func TestQueue_Close(t *testing.T) {
	q, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	q.Close()

	if q.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", q.Len())
	}

	_, ok := <-q.Jobs()

	if ok {
		t.Fatal("Jobs() channel should be closed")
	}
}

func TestQueue_DoubleCloseIsSafe(t *testing.T) {
	q, err := New(1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	q.Close()
	q.Close()
}
