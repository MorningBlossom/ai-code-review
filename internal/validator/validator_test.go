package validator

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestFindingValidator_Validate(t *testing.T) {
	validator := NewFindingValidator()

	findings := []review.ReviewFinding{
		{
			FindingID:   "security:auth.go:12",
			Source:      "gosec",
			Category:    "security",
			Severity:    "high",
			Confidence:  1.0,
			Title:       "Potential security issue",
			Explanation: "User input reaches authentication logic without validation.",
			Suggestion:  "Validate the input before using it.",
			FilePath:    "auth.go",
			StartLine:   12,
			EndLine:     12,
			Evidence:    "user input reaches authentication logic",
		},
	}

	result, err := validator.Validate(
		context.Background(),
		review.ReviewRequest{},
		findings,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result))
	}

	if result[0].FindingID != "security:auth.go:12" {
		t.Fatalf(
			"unexpected finding ID: %s",
			result[0].FindingID,
		)
	}
}

func TestFindingValidator_RejectsInvalidFindings(t *testing.T) {
	validator := NewFindingValidator()

	tests := []struct {
		name    string
		finding review.ReviewFinding
	}{
		{
			name: "missing finding ID",
			finding: review.ReviewFinding{
				Source:      "gosec",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Security issue",
				Explanation: "Potential security problem.",
				FilePath:    "auth.go",
			},
		},
		{
			name: "invalid category",
			finding: review.ReviewFinding{
				FindingID:   "finding-1",
				Source:      "model",
				Category:    "unknown",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Issue",
				Explanation: "Potential problem.",
				FilePath:    "auth.go",
			},
		},
		{
			name: "invalid severity",
			finding: review.ReviewFinding{
				FindingID:   "finding-2",
				Source:      "model",
				Category:    "security",
				Severity:    "unknown",
				Confidence:  1.0,
				Title:       "Issue",
				Explanation: "Potential problem.",
				FilePath:    "auth.go",
			},
		},
		{
			name: "invalid confidence",
			finding: review.ReviewFinding{
				FindingID:   "finding-3",
				Source:      "model",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.5,
				Title:       "Issue",
				Explanation: "Potential problem.",
				FilePath:    "auth.go",
			},
		},
		{
			name: "absolute file path",
			finding: review.ReviewFinding{
				FindingID:   "finding-4",
				Source:      "model",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Issue",
				Explanation: "Potential problem.",
				FilePath:    "/tmp/auth.go",
			},
		},
		{
			name: "parent traversal",
			finding: review.ReviewFinding{
				FindingID:   "finding-5",
				Source:      "model",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Issue",
				Explanation: "Potential problem.",
				FilePath:    "../auth.go",
			},
		},
		{
			name: "invalid line range",
			finding: review.ReviewFinding{
				FindingID:   "finding-6",
				Source:      "model",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Issue",
				Explanation: "Potential problem.",
				FilePath:    "auth.go",
				StartLine:   20,
				EndLine:     10,
			},
		},
	}

	result, err := validator.Validate(
		context.Background(),
		review.ReviewRequest{},
		testsToFindings(tests),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf(
			"expected all invalid findings to be rejected, got %d",
			len(result),
		)
	}
}

func TestFindingValidator_DeduplicatesFindings(t *testing.T) {
	validator := NewFindingValidator()

	findings := []review.ReviewFinding{
		{
			FindingID:   "model-1",
			Source:      "model",
			Category:    "security",
			Severity:    "medium",
			Confidence:  0.70,
			Title:       "Potential security issue",
			Explanation: "Potential issue.",
			Suggestion:  "Validate the input.",
			FilePath:    "auth.go",
			StartLine:   12,
			EndLine:     12,
		},
		{
			FindingID:   "gosec-1",
			Source:      "gosec",
			Category:    "security",
			Severity:    "high",
			Confidence:  1.0,
			Title:       "Potential security issue",
			Explanation: "Confirmed by deterministic analyzer.",
			Suggestion:  "Validate the input.",
			FilePath:    "auth.go",
			StartLine:   12,
			EndLine:     12,
		},
	}

	result, err := validator.Validate(
		context.Background(),
		review.ReviewRequest{},
		findings,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf(
			"expected 1 deduplicated finding, got %d",
			len(result),
		)
	}

	if result[0].FindingID != "gosec-1" {
		t.Fatalf(
			"expected higher-confidence finding to remain, got %s",
			result[0].FindingID,
		)
	}
}

func TestFindingValidator_KeepsDifferentFindings(t *testing.T) {
	validator := NewFindingValidator()

	findings := []review.ReviewFinding{
		{
			FindingID:   "finding-1",
			Source:      "model",
			Category:    "security",
			Severity:    "high",
			Confidence:  0.9,
			Title:       "Authentication issue",
			Explanation: "Authentication input is not validated.",
			FilePath:    "auth.go",
			StartLine:   12,
			EndLine:     12,
		},
		{
			FindingID:   "finding-2",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.9,
			Title:       "Incorrect error handling",
			Explanation: "The returned error is ignored.",
			FilePath:    "auth.go",
			StartLine:   20,
			EndLine:     20,
		},
	}

	result, err := validator.Validate(
		context.Background(),
		review.ReviewRequest{},
		findings,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf(
			"expected 2 different findings, got %d",
			len(result),
		)
	}
}

func TestFindingValidator_ContextCancellation(t *testing.T) {
	validator := NewFindingValidator()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := validator.Validate(
		ctx,
		review.ReviewRequest{},
		[]review.ReviewFinding{
			{
				FindingID:   "finding-1",
				Source:      "model",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Issue",
				Explanation: "Potential issue.",
				FilePath:    "auth.go",
			},
		},
	)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func testsToFindings(
	tests []struct {
		name    string
		finding review.ReviewFinding
	},
) []review.ReviewFinding {
	findings := make([]review.ReviewFinding, 0, len(tests))

	for _, test := range tests {
		findings = append(findings, test.finding)
	}

	return findings
}
