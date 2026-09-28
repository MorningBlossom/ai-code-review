package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type GithubWebhookHandler struct {
	orchestrator  orchestrator.Orchestrator
	webhookSecret string
	deliveryStore *DeliveryStore
}

func NewGithubWebhookHandler(orchestrator orchestrator.Orchestrator, webhook string, store *DeliveryStore) *GithubWebhookHandler {
	return &GithubWebhookHandler{
		orchestrator:  orchestrator,
		webhookSecret: webhook,
		deliveryStore: store,
	}
}

type pullRequestWebhookPayload struct {
	Action       string `json:"action"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		User   struct {
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

func (h *GithubWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")

	if err := github.VerifyWebhookSignature(
		payload,
		signature,
		h.webhookSecret,
	); err != nil {
		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	if eventType == "" {
		http.Error(w, "missing GitHub event type", http.StatusBadRequest)
		return
	}

	if deliveryID == "" {
		http.Error(w, "missing GitHub delivery ID", http.StatusBadRequest)
		return
	}

	if h.deliveryStore.Seen(deliveryID) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if eventType != "pull_request" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var event pullRequestWebhookPayload

	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(w, "invalid webhook payload", http.StatusBadRequest)
		return
	}

	if event.Installation.ID == 0 {
		http.Error(w, "missing installation ID", http.StatusBadRequest)
		return
	}

	if event.Repository.Owner.Login == "" {
		http.Error(w, "missing repository owner", http.StatusBadRequest)
		return
	}

	if event.Repository.Name == "" {
		http.Error(w, "missing repository name", http.StatusBadRequest)
		return
	}

	if event.PullRequest.Number <= 0 {
		http.Error(w, "missing pull request number", http.StatusBadRequest)
		return
	}

	if event.PullRequest.Head.SHA == "" {
		http.Error(w, "missing pull request head SHA", http.StatusBadRequest)
		return
	}

	if event.PullRequest.Base.SHA == "" {
		http.Error(w, "missing pull request base SHA", http.StatusBadRequest)
		return
	}

	switch event.Action {
	case "opened", "synchronize", "reopened", "ready_for_review":
		// Supported actions.
	default:
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
		EventType:           eventType,
		RequestedBy:         event.PullRequest.User.Login,
		RequestedAt:         time.Now().UTC(),
		ReviewMode:          "pull_request",
		ReviewPolicyVersion: "v1",
	}

	_, err = h.orchestrator.Review(r.Context(), request)
	if err != nil {
		http.Error(w, "review failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
