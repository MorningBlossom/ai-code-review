package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type LocalProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewLocalProvider(
	config Config,
) *LocalProvider {
	timeout := time.Duration(config.TimeoutSeconds) * time.Second

	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	return &LocalProvider{
		baseURL: strings.TrimRight(
			config.BaseURL,
			"/",
		),
		model: config.Model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (p *LocalProvider) Name() string {
	return "local"
}

func (p *LocalProvider) Review(
	ctx context.Context,
	reviewContext review.ReviewContext,
) ([]review.ReviewFinding, error) {
	prompt := buildReviewPrompt(reviewContext)

	requestBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: reviewSystemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal model request: %w",
			err,
		)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/v1/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create model request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	response, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"call local model: %w",
			err,
		)
	}

	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"local model returned HTTP %d",
			response.StatusCode,
		)
	}

	var result chatResponse

	if err := json.NewDecoder(
		response.Body,
	).Decode(&result); err != nil {
		return nil, fmt.Errorf(
			"decode model response: %w",
			err,
		)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf(
			"local model returned no choices",
		)
	}

	content := strings.TrimSpace(
		result.Choices[0].Message.Content,
	)

	if content == "" {
		return nil, fmt.Errorf(
			"local model returned empty content",
		)
	}

	return parseFindings(content)
}

const reviewSystemPrompt = `
You are a code review engine.

Review the supplied pull request context.

Focus on:
- correctness
- security
- concurrency
- reliability
- error handling
- maintainability
- test quality
- Go and microservice-specific problems

Do not report:
- stylistic preferences without engineering impact
- issues unrelated to the supplied code
- speculative vulnerabilities without evidence

Return ONLY valid JSON.
`
