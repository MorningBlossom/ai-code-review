package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type findingResponse struct {
	Findings []review.ReviewFinding `json:"findings"`
}

func parseFindings(
	content string,
) ([]review.ReviewFinding, error) {
	content = strings.TrimSpace(content)

	content = strings.TrimPrefix(
		content,
		"```json",
	)

	content = strings.TrimPrefix(
		content,
		"```",
	)

	content = strings.TrimSuffix(
		content,
		"```",
	)

	content = strings.TrimSpace(content)

	var response findingResponse

	if err := json.Unmarshal(
		[]byte(content),
		&response,
	); err != nil {
		return nil, fmt.Errorf(
			"parse model findings: %w",
			err,
		)
	}

	for i := range response.Findings {
		response.Findings[i].Source = "local-model"
	}

	return response.Findings, nil
}
