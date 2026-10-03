package model

import (
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestBuildReviewPrompt_IncludesAnalyzerFindings(t *testing.T) {
	reviewContext := review.ReviewContext{
		PullRequestTitle: "Add authentication",
		PullRequestBody:  "Add authentication middleware.",
		SourceFiles: []review.SourceFile{
			{
				Path:    "auth.go",
				Content: "func authenticate() {}",
			},
		},
		AnalyzerFindings: []review.ReviewFinding{
			{
				FindingID:   "gosec:auth.go",
				Source:      "gosec",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Potential security issue",
				Explanation: "Authentication input is not validated.",
				Suggestion:  "Validate the authentication input before use.",
				FilePath:    "auth.go",
				StartLine:   12,
				EndLine:     12,
				Evidence:    "user input reaches authentication logic",
			},
		},
	}

	prompt := buildReviewPrompt(reviewContext)

	expectedStrings := []string{
		"Deterministic Analyzer Findings:",
		"gosec",
		"Severity: high",
		"Confidence: 1.00",
		"Category: security",
		"Title: Potential security issue",
		"File: auth.go",
		"Lines: 12-12",
		"Explanation: Authentication input is not validated.",
		"Suggestion: Validate the authentication input before use.",
		"Evidence:",
		"user input reaches authentication logic",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(prompt, expected) {
			t.Fatalf(
				"expected prompt to contain %q\nprompt:\n%s",
				expected,
				prompt,
			)
		}
	}
}


func TestBuildReviewPrompt_IncludesSecurityDataFlowGuidance(t *testing.T) {
	reviewContext := review.ReviewContext{
		PullRequestTitle: "Harden service generation",
		SourceFiles: []review.SourceFile{
			{
				Path: "generator.go",
				Content: "func GenerateService(serviceName string) error { return nil }",
			},
		},
	}

	prompt := buildReviewPrompt(reviewContext)

	expectedStrings := []string{
		"Trace data flow from untrusted or externally influenced inputs to sensitive sinks",
		"verify whether that value is itself attacker-controlled",
		"path traversal",
		"attacker-controlled roots",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(prompt, expected) {
			t.Fatalf(
				"expected security guidance %q in prompt\nprompt:\n%s",
				expected,
				prompt,
			)
		}
	}
}
