package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const (
	maxWebhookPayloadSize = 5 << 20 // 5 MiB
	manualReviewCommand   = "@mb-ai"
)

type GithubWebhookHandler struct {
	orchestrator        orchestrator.Orchestrator
	github              github.Provider
	webhookSecret       string
	deliveryStore       *DeliveryStore
	allowedOrganization string
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
	if r.Method != http.MethodPost {
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
		http.Error(
			w,
			"invalid webhook signature",
			http.StatusUnauthorized,
		)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

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
		h.handlePullRequestEvent(
			w,
			r,
			payload,
			deliveryID,
		)

	case "issue_comment":
		h.handleIssueCommentEvent(
			w,
			r,
			payload,
			deliveryID,
		)

	default:
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
			w.WriteHeader(http.StatusAccepted)
			return
		}

	case "ready_for_review":
		if event.PullRequest.Draft {
			w.WriteHeader(http.StatusAccepted)
			return
		}

	case "reopened":
		if event.PullRequest.Draft {
			w.WriteHeader(http.StatusAccepted)
			return
		}

	case "synchronize":
		// New commits must NOT automatically trigger an AI review.
		w.WriteHeader(http.StatusAccepted)
		return

	default:
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

	_, err := h.orchestrator.Review(
		r.Context(),
		request,
	)
	if err != nil {
		h.deliveryStore.Forget(deliveryID)

		http.Error(
			w,
			"review failed",
			http.StatusInternalServerError,
		)
		return
	}

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

	// issue_comment is emitted for normal Issues as well as PRs.
	if event.Issue.PullRequest == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	command := strings.ToLower(
		strings.TrimSpace(event.Comment.Body),
	)

	if command != manualReviewCommand {
		w.WriteHeader(http.StatusAccepted)
		return
	}

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

	/*
	   Always fetch the current PR.

	   This ensures @mb-ai reviews the latest commit rather than
	   using potentially stale information from the issue_comment
	   payload.
	*/
	pr, err := h.github.GetPullRequest(
		r.Context(),
		event.Installation.ID,
		event.Repository.Owner.Login,
		event.Repository.Name,
		event.Issue.Number,
	)
	if err != nil {
		h.deliveryStore.Forget(deliveryID)

		http.Error(
			w,
			"failed to fetch pull request",
			http.StatusInternalServerError,
		)
		return
	}

	// @mb-ai is ignored while the PR remains a draft.
	if pr.Draft {
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
		PullRequestNumber:   pr.Number,
		BaseSHA:             pr.BaseSHA,
		HeadSHA:             pr.HeadSHA,
		EventType:           "issue_comment",
		RequestedBy:         event.Comment.User.Login,
		RequestedAt:         time.Now().UTC(),
		ReviewMode:          "manual_full_pr",
		ReviewPolicyVersion: "v1",
	}

	_, err = h.orchestrator.Review(
		r.Context(),
		request,
	)
	if err != nil {
		h.deliveryStore.Forget(deliveryID)

		http.Error(
			w,
			"review failed",
			http.StatusInternalServerError,
		)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
