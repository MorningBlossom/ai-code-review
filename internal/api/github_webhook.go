package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const (
	maxWebhookPayloadSize  = 5 << 20 // 5 MiB
	manualReviewCommand    = "@mb-ai"
	backgroundReviewTimeout = 15 * time.Minute
)

type GithubWebhookHandler struct {
	orchestrator        orchestrator.Orchestrator
	github              github.Provider
	webhookSecret       string
	deliveryStore       *DeliveryStore
	allowedOrganization string
	backgroundRunner    func(func())
}

func NewGithubWebhookHandler(
	orchestrator orchestrator.Orchestrator,
	githubProvider github.Provider,
	webhook string,
	store *DeliveryStore,
	organisation string,
) *GithubWebhookHandler {
	return &GithubWebhookHandler{
		orchestrator:        orchestrator,
		github:              githubProvider,
		webhookSecret:       webhook,
		deliveryStore:       store,
		allowedOrganization: organisation,
		backgroundRunner: func(fn func()) {
			go fn()
		},
	}
}

type pullRequestWebhookPayload struct {
	Action string `json:"action"`

	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`

	Repository struct {
		Name string `json:"name"`

		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`

	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Draft  bool   `json:"draft"`

		User struct {
			Login string `json:"login"`
		} `json:"user"`

		Base struct {
			SHA string `json:"sha"`
		} `json:"base"`

		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
}

type issueCommentWebhookPayload struct {
	Action string `json:"action"`

	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`

	Repository struct {
		Name string `json:"name"`

		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`

	Issue struct {
		Number int `json:"number"`

		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	} `json:"issue"`

	Comment struct {
		Body string `json:"body"`

		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
}

func (h *GithubWebhookHandler) HandleWebhook(
	w http.ResponseWriter,
	r *http.Request,
) {

	log.Printf(
		"webhook received method=%s path=%s remote=%s",
		r.Method,
		r.URL.Path,
		r.RemoteAddr,
	)

	if r.Method != http.MethodPost {
		log.Printf(
			"webhook rejected reason=method_not_allowed method=%s",
			r.Method,
		)
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	payload, err := io.ReadAll(
		io.LimitReader(
			r.Body,
			maxWebhookPayloadSize+1,
		),
	)
	log.Printf(
		"webhook payload accepted size=%d",
		len(payload),
	)
	if err != nil {
		http.Error(
			w,
			"failed to read request body",
			http.StatusBadRequest,
		)
		return
	}

	if len(payload) > maxWebhookPayloadSize {
		http.Error(
			w,
			"request body too large",
			http.StatusRequestEntityTooLarge,
		)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")

	if err := github.VerifyWebhookSignature(
		payload,
		signature,
		h.webhookSecret,
	); err != nil {
		log.Printf(
			"webhook rejected reason=invalid_signature delivery=%s",
			r.Header.Get("X-GitHub-Delivery"),
		)
		http.Error(
			w,
			"invalid webhook signature",
			http.StatusUnauthorized,
		)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	log.Printf(
		"webhook headers event=%s delivery=%s",
		eventType,
		deliveryID,
	)

	if eventType == "" {
		http.Error(
			w,
			"missing GitHub event type",
			http.StatusBadRequest,
		)
		return
	}

	if deliveryID == "" {
		http.Error(
			w,
			"missing GitHub delivery ID",
			http.StatusBadRequest,
		)
		return
	}

	switch eventType {
	case "pull_request":

		log.Printf(
			"webhook dispatch event=pull_request delivery=%s",
			deliveryID,
		)

		h.handlePullRequestEvent(
			w,
			r,
			payload,
			deliveryID,
		)

	case "issue_comment":

		log.Printf(
			"webhook dispatch event=issue_comment delivery=%s",
			deliveryID,
		)

		h.handleIssueCommentEvent(
			w,
			r,
			payload,
			deliveryID,
		)

	default:
		log.Printf(
			"webhook ignored reason=unsupported_event event=%s delivery=%s",
			eventType,
			deliveryID,
		)
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *GithubWebhookHandler) handlePullRequestEvent(
	w http.ResponseWriter,
	r *http.Request,
	payload []byte,
	deliveryID string,
) {
	var event pullRequestWebhookPayload

	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(
			w,
			"invalid webhook payload",
			http.StatusBadRequest,
		)
		return
	}

	log.Printf(
		"pull_request event parsed action=%s org=%s repo=%s pr=%d draft=%t delivery=%s",
		event.Action,
		event.Repository.Owner.Login,
		event.Repository.Name,
		event.PullRequest.Number,
		event.PullRequest.Draft,
		deliveryID,
	)

	if event.Installation.ID == 0 {
		http.Error(
			w,
			"missing installation ID",
			http.StatusBadRequest,
		)
		return
	}

	if event.Repository.Owner.Login != h.allowedOrganization {
		http.Error(
			w,
			"unauthorized organization",
			http.StatusUnauthorized,
		)
		return
	}

	if event.Repository.Name == "" {
		http.Error(
			w,
			"missing repository name",
			http.StatusBadRequest,
		)
		return
	}

	if event.PullRequest.Number <= 0 {
		http.Error(
			w,
			"missing pull request number",
			http.StatusBadRequest,
		)
		return
	}

	if event.PullRequest.Head.SHA == "" {
		http.Error(
			w,
			"missing pull request head SHA",
			http.StatusBadRequest,
		)
		return
	}

	if event.PullRequest.Base.SHA == "" {
		http.Error(
			w,
			"missing pull request base SHA",
			http.StatusBadRequest,
		)
		return
	}

	/*
	   Automatic review policy:

	   opened:
	      Review only when PR is ready.

	   ready_for_review:
	      Always review.

	   reopened:
	      Review only when PR is ready.

	   synchronize:
	      Never automatically review.

	   All other actions:
	      Ignore.
	*/
	switch event.Action {
	case "opened":
		if event.PullRequest.Draft {

			log.Printf(
				"review ignored reason=draft_pr action=opened org=%s repo=%s pr=%d",
				event.Repository.Owner.Login,
				event.Repository.Name,
				event.PullRequest.Number,
			)

			w.WriteHeader(http.StatusAccepted)
			return
		}

	case "ready_for_review":
		if event.PullRequest.Draft {

			log.Printf(
				"review ignored reason=draft_pr action=ready_for_review org=%s repo=%s pr=%d",
				event.Repository.Owner.Login,
				event.Repository.Name,
				event.PullRequest.Number,
			)

			w.WriteHeader(http.StatusAccepted)
			return
		}

	case "reopened":
		if event.PullRequest.Draft {
			log.Printf(
				"review ignored reason=draft_pr action=reopened org=%s repo=%s pr=%d",
				event.Repository.Owner.Login,
				event.Repository.Name,
				event.PullRequest.Number,
			)
			w.WriteHeader(http.StatusAccepted)
			return
		}

	case "synchronize":
		log.Printf(
			"review ignored reason=synchronize_not_auto_reviewed org=%s repo=%s pr=%d",
			event.Repository.Owner.Login,
			event.Repository.Name,
			event.PullRequest.Number,
		)
		// New commits must NOT automatically trigger an AI review.
		w.WriteHeader(http.StatusAccepted)
		return

	default:
		log.Printf(
			"review ignored reason=unsupported_pull_request_action action=%s org=%s repo=%s pr=%d",
			event.Action,
			event.Repository.Owner.Login,
			event.Repository.Name,
			event.PullRequest.Number,
		)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if h.deliveryStore.Seen(deliveryID) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	request := review.ReviewRequest{
		ReviewID:            deliveryID,
		InstallationID:      event.Installation.ID,
		Organization:        event.Repository.Owner.Login,
		Repository:          event.Repository.Name,
		PullRequestNumber:   event.PullRequest.Number,
		BaseSHA:             event.PullRequest.Base.SHA,
		HeadSHA:             event.PullRequest.Head.SHA,
		EventType:           "pull_request",
		RequestedBy:         event.PullRequest.User.Login,
		RequestedAt:         time.Now().UTC(),
		ReviewMode:          "pull_request",
		ReviewPolicyVersion: "v1",
	}

	log.Printf(
		"review queued mode=pull_request review_id=%s org=%s repo=%s pr=%d head=%s",
		request.ReviewID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
		request.HeadSHA,
	)

	h.startBackgroundReview(request, deliveryID)

	// GitHub expects the webhook endpoint to acknowledge deliveries quickly.
	// The review itself can take much longer because it runs analyzers, the
	// model, validation, and GitHub publishing.
	w.WriteHeader(http.StatusAccepted)
}

func (h *GithubWebhookHandler) handleIssueCommentEvent(
	w http.ResponseWriter,
	r *http.Request,
	payload []byte,
	deliveryID string,
) {
	var event issueCommentWebhookPayload

	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(
			w,
			"invalid webhook payload",
			http.StatusBadRequest,
		)
		return
	}

	log.Printf(
		"issue_comment event parsed action=%s org=%s repo=%s pr=%d user=%s delivery=%s",
		event.Action,
		event.Repository.Owner.Login,
		event.Repository.Name,
		event.Issue.Number,
		event.Comment.User.Login,
		deliveryID,
	)

	// issue_comment is emitted for normal Issues as well as PRs.
	if event.Issue.PullRequest == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	command := strings.ToLower(
		strings.TrimSpace(event.Comment.Body),
	)

	if command != manualReviewCommand {
		log.Printf(
			"manual review ignored reason=command_not_matched org=%s repo=%s pr=%d command=%q",
			event.Repository.Owner.Login,
			event.Repository.Name,
			event.Issue.Number,
			command,
		)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	log.Printf(
		"manual review command accepted command=%s org=%s repo=%s pr=%d delivery=%s",
		manualReviewCommand,
		event.Repository.Owner.Login,
		event.Repository.Name,
		event.Issue.Number,
		deliveryID,
	)

	if event.Installation.ID == 0 {
		http.Error(
			w,
			"missing installation ID",
			http.StatusBadRequest,
		)
		return
	}

	if event.Repository.Owner.Login != h.allowedOrganization {
		http.Error(
			w,
			"unauthorized organization",
			http.StatusUnauthorized,
		)
		return
	}

	if event.Repository.Name == "" {
		http.Error(
			w,
			"missing repository name",
			http.StatusBadRequest,
		)
		return
	}

	if event.Issue.Number <= 0 {
		http.Error(
			w,
			"missing pull request number",
			http.StatusBadRequest,
		)
		return
	}

	if h.github == nil {
		http.Error(
			w,
			"GitHub provider is not configured",
			http.StatusInternalServerError,
		)
		return
	}

	if h.deliveryStore.Seen(deliveryID) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	request := review.ReviewRequest{
		ReviewID:            deliveryID,
		InstallationID:      event.Installation.ID,
		Organization:        event.Repository.Owner.Login,
		Repository:          event.Repository.Name,
		PullRequestNumber:   event.Issue.Number,
		EventType:           "issue_comment",
		RequestedBy:         event.Comment.User.Login,
		RequestedAt:         time.Now().UTC(),
		ReviewMode:          "manual_full_pr",
		ReviewPolicyVersion: "v1",
	}

	log.Printf(
		"manual review queued mode=manual_full_pr review_id=%s org=%s repo=%s pr=%d requested_by=%s",
		request.ReviewID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
		request.RequestedBy,
	)

	h.backgroundRunner(func() {
		h.runManualReview(request, deliveryID)
	})

	// Acknowledge the GitHub delivery immediately. Fetching the current PR and
	// running the review happen in the background.
	w.WriteHeader(http.StatusAccepted)
}


func (h *GithubWebhookHandler) startBackgroundReview(
	request review.ReviewRequest,
	deliveryID string,
) {
	h.backgroundRunner(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			backgroundReviewTimeout,
		)
		defer cancel()

		_, err := h.orchestrator.Review(
			ctx,
			request,
		)
		if err != nil {
			log.Printf(
				"background review failed mode=%s review_id=%s org=%s repo=%s pr=%d error=%v",
				request.ReviewMode,
				request.ReviewID,
				request.Organization,
				request.Repository,
				request.PullRequestNumber,
				err,
			)
			h.deliveryStore.Forget(deliveryID)
			return
		}

		log.Printf(
			"background review completed mode=%s review_id=%s org=%s repo=%s pr=%d",
			request.ReviewMode,
			request.Organization,
			request.Repository,
			request.PullRequestNumber,
		)
	})
}

func (h *GithubWebhookHandler) runManualReview(
	request review.ReviewRequest,
	deliveryID string,
) {
	log.Printf(
		"manual review fetching current PR org=%s repo=%s pr=%d",
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)

	pr, err := h.github.GetPullRequest(
		context.Background(),
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		log.Printf(
			"manual review failed stage=get_pull_request review_id=%s org=%s repo=%s pr=%d error=%v",
			request.ReviewID,
			request.Organization,
			request.Repository,
			request.PullRequestNumber,
			err,
		)
		h.deliveryStore.Forget(deliveryID)
		return
	}

	log.Printf(
		"manual review PR fetched org=%s repo=%s pr=%d draft=%t head=%s",
		request.Organization,
		request.Repository,
		pr.Number,
		pr.Draft,
		pr.HeadSHA,
	)

	if pr.Draft {
		log.Printf(
			"manual review ignored reason=draft_pr org=%s repo=%s pr=%d",
			request.Organization,
			request.Repository,
			pr.Number,
		)
		h.deliveryStore.Forget(deliveryID)
		return
	}

	request.BaseSHA = pr.BaseSHA
	request.HeadSHA = pr.HeadSHA
	request.PullRequestNumber = pr.Number

	h.startBackgroundReview(request, deliveryID)
}
