package api

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

const maxReviewPayloadSize = 1 << 20 // 1 MiB

type Handler struct {
	orchestrator        orchestrator.Orchestrator
	apiToken            string
	allowedOrganization string
}

func NewHandler(
	orchestrator orchestrator.Orchestrator,
	apiToken string,
	allowedOrganization string,
) *Handler {
	return &Handler{
		orchestrator:        orchestrator,
		apiToken:            apiToken,
		allowedOrganization: allowedOrganization,
	}
}

func (h Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	if !h.authenticate(r) {
		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)
		return
	}

	body, err := io.ReadAll(
		io.LimitReader(
			r.Body,
			maxReviewPayloadSize+1,
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

	if len(body) > maxReviewPayloadSize {
		http.Error(
			w,
			"request body too large",
			http.StatusRequestEntityTooLarge,
		)
		return
	}

	var request review.ReviewRequest

	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if request.ReviewID == "" {
		http.Error(
			w,
			"review_id is required",
			http.StatusBadRequest,
		)
		return
	}

	if request.Organization == "" {
		http.Error(
			w,
			"organization is required",
			http.StatusBadRequest,
		)
		return
	}

	if request.Organization != h.allowedOrganization {
		http.Error(
			w,
			"unauthorized organization",
			http.StatusUnauthorized,
		)
		return
	}

	if request.Repository == "" {
		http.Error(
			w,
			"repository is required",
			http.StatusBadRequest,
		)
		return
	}

	if request.PullRequestNumber <= 0 {
		http.Error(
			w,
			"pull_request_number must be greater than 0",
			http.StatusBadRequest,
		)
		return
	}

	if request.HeadSHA == "" {
		http.Error(
			w,
			"head_sha is required",
			http.StatusBadRequest,
		)
		return
	}

	result, err := h.orchestrator.Review(
		r.Context(),
		request,
	)
	if err != nil {
		log.Printf(
			"review failed: review_id=%s organization=%s repository=%s pr=%d error=%v",
			request.ReviewID,
			request.Organization,
			request.Repository,
			request.PullRequestNumber,
			err,
		)

		http.Error(
			w,
			"review failed",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf(
			"failed to encode review response: review_id=%s error=%v",
			request.ReviewID,
			err,
		)

		// The response may already have been partially written,
		// so there is no reliable status change we can make here.
		return
	}
}

func (h Handler) authenticate(r *http.Request) bool {
	if h.apiToken == "" {
		return false
	}

	const prefix = "Bearer "

	authorization := r.Header.Get("Authorization")

	if !strings.HasPrefix(authorization, prefix) {
		return false
	}

	token := strings.TrimSpace(
		strings.TrimPrefix(
			authorization,
			prefix,
		),
	)

	if token == "" {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(token),
		[]byte(h.apiToken),
	) == 1
}

func (h *Handler) Health(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(
		map[string]string{
			"status": "ok",
		},
	)
}
