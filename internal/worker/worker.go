package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/queue"
)

const (
	defaultConcurrency   = 1
	defaultReviewTimeout = 5 * time.Minute
)

type DeliveryTracker interface {
	Forget(deliveryID string)
}

type Worker struct {
	queue         *queue.Queue
	orchestrator  orchestrator.Orchestrator
	deliveryStore DeliveryTracker
	concurrency   int
	reviewTimeout time.Duration

	startOnce sync.Once
	waitGroup sync.WaitGroup
}

type Config struct {
	Queue         *queue.Queue
	Orchestrator  orchestrator.Orchestrator
	DeliveryStore DeliveryTracker

	Concurrency   int
	ReviewTimeout time.Duration
}

func New(config Config) (*Worker, error) {
	if config.Queue == nil {
		return nil, errors.New("queue is required")
	}

	if config.Orchestrator == nil {
		return nil, errors.New("orchestrator is required")
	}

	if config.DeliveryStore == nil {
		return nil, errors.New("delivery store is required")
	}

	if config.Concurrency <= 0 {
		config.Concurrency = defaultConcurrency
	}

	if config.ReviewTimeout <= 0 {
		config.ReviewTimeout = defaultReviewTimeout
	}

	return &Worker{
		queue:         config.Queue,
		orchestrator:  config.Orchestrator,
		deliveryStore: config.DeliveryStore,
		concurrency:   config.Concurrency,
		reviewTimeout: config.ReviewTimeout,
	}, nil
}

func (w *Worker) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	w.startOnce.Do(func() {
		w.waitGroup.Add(w.concurrency)

		for i := 0; i < w.concurrency; i++ {
			workerID := i + 1

			go func() {
				defer w.waitGroup.Done()

				w.runWorker(ctx, workerID)
			}()
		}
	})
}

func (w *Worker) Wait() {
	w.waitGroup.Wait()
}

func (w *Worker) runWorker(ctx context.Context, workerID int) {
	log.Printf(
		"review worker started: worker_id=%d",
		workerID,
	)

	defer func() {
		log.Printf(
			"review worker stopped: worker_id=%d",
			workerID,
		)
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-w.queue.Jobs():
			if !ok {
				return
			}

			w.process(ctx, workerID, job)
		}
	}
}

func (w *Worker) process(
	parentContext context.Context,
	workerID int,
	job queue.Job,
) {
	log.Printf(
		"review worker processing: worker_id=%d review_id=%s organization=%s repository=%s pr=%d",
		workerID,
		job.Request.ReviewID,
		job.Request.Organization,
		job.Request.Repository,
		job.Request.PullRequestNumber,
	)

	reviewContext, cancel := context.WithTimeout(
		parentContext,
		w.reviewTimeout,
	)
	defer cancel()

	_, err := w.orchestrator.Review(
		reviewContext,
		job.Request,
	)

	if err != nil {
		log.Printf(
			"review worker failed: worker_id=%d review_id=%s error=%v",
			workerID,
			job.Request.ReviewID,
			err,
		)

		// Remove the delivery from the deduplication store.
		//
		// GitHub can then retry the webhook and the review can be
		// attempted again.
		w.deliveryStore.Forget(job.DeliveryID)

		return
	}

	log.Printf(
		"review worker completed: worker_id=%d review_id=%s",
		workerID,
		job.Request.ReviewID,
	)
}
