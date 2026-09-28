package api

import (
	"encoding/json"
	"net/http"

	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Handler struct {
	orchestrator orchestrator.Orchestrator
}

func NewHandler(orchestrator orchestrator.Orchestrator) *Handler {
	return &Handler{
		orchestrator: orchestrator,
	}
}

func (h Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request review.ReviewRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if request.ReviewID == "" {
		http.Error(w, "review_id is required", http.StatusBadRequest)
		return
	}

	if request.Organization == "" {
		http.Error(w, "organization is required", http.StatusBadRequest)
		return
	}

	if request.Repository == "" {
		http.Error(w, "repository is required", http.StatusBadRequest)
		return
	}

	if request.PullRequestNumber <= 0 {
		http.Error(w, "pull_request_number must be greater than 0", http.StatusBadRequest)
		return
	}

	if request.HeadSHA == "" {
		http.Error(w, "head_sha is required", http.StatusBadRequest)
		return
	}

	result, err := h.orchestrator.Review(r.Context(), request)
	if err != nil {
		http.Error(w, "review failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
