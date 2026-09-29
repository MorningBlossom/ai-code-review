package validator

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestFindingValidator_ValidateWithContext_RetainsValidFinding(t *testing.T) {
	validator := &FindingValidator{}

	request := review.ReviewRequest{
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 1,
	}

	reviewContext := review.ReviewContext{
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "main.go",
				Status: "modified",
				Patch:  "@@ -1,2 +1,3 @@\n package main\n+\n func main() {}\n",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
	}

	findings := []review.ReviewFinding{
		{
			FindingID:   "finding-1",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.95,
			Title:       "Potential issue",
			Explanation: "Potential correctness issue.",
			Suggestion:  "Fix the issue.",
			FilePath:    "main.go",
			StartLine:   3,
			EndLine:     3,
		},
	}

	result, err := validator.ValidateWithContext(
		context.Background(),
		request,
		reviewContext,
		findings,
	)
	if err != nil {
		t.Fatalf("ValidateWithContext returned error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result))
	}

	if result[0].FindingID != "finding-1" {
		t.Fatalf("expected finding-1, got %q", result[0].FindingID)
	}
}

func TestFindingValidator_ValidateWithContext_RemovesUnknownFile(t *testing.T) {
	validator := &FindingValidator{}

	request := review.ReviewRequest{
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 1,
	}

	reviewContext := review.ReviewContext{
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "main.go",
				Status: "modified",
				Patch:  "@@ -1,2 +1,3 @@\n package main\n+\n func main() {}\n",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
	}

	findings := []review.ReviewFinding{
		{
			FindingID:   "unknown-file",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.9,
			Title:       "Unknown file",
			Explanation: "The finding references a file that is not present in the review context.",
			FilePath:    "does-not-exist.go",
			StartLine:   1,
			EndLine:     1,
		},
	}

	result, err := validator.ValidateWithContext(
		context.Background(),
		request,
		reviewContext,
		findings,
	)
	if err != nil {
		t.Fatalf("ValidateWithContext returned error: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf(
			"expected unknown-file finding to be removed, got %d findings",
			len(result),
		)
	}
}

func TestFindingValidator_ValidateWithContext_RemovesInvalidLineRange(t *testing.T) {
	validator := &FindingValidator{}

	request := review.ReviewRequest{
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 1,
	}

	reviewContext := review.ReviewContext{
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "main.go",
				Status: "modified",
				Patch:  "@@ -1,2 +1,3 @@\n package main\n+\n func main() {}\n",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
	}

	findings := []review.ReviewFinding{
		{
			FindingID:   "invalid-line",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.9,
			Title:       "Invalid line",
			Explanation: "The finding references an invalid line range.",
			FilePath:    "main.go",
			StartLine:   100,
			EndLine:     101,
		},
	}

	result, err := validator.ValidateWithContext(
		context.Background(),
		request,
		reviewContext,
		findings,
	)
	if err != nil {
		t.Fatalf("ValidateWithContext returned error: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf(
			"expected invalid-line finding to be removed, got %d findings",
			len(result),
		)
	}
}

func TestFindingValidator_ValidateWithContext_RemovesFindingOutsideChangedLines(t *testing.T) {
	validator := &FindingValidator{}

	request := review.ReviewRequest{
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 1,
	}

	reviewContext := review.ReviewContext{
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "main.go",
				Status: "modified",
				Patch:  "@@ -2,1 +2,2 @@\n \n+func main() {}\n",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
	}

	findings := []review.ReviewFinding{
		{
			FindingID:   "unchanged-line",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.9,
			Title:       "Unchanged line",
			Explanation: "The finding references an unchanged line.",
			FilePath:    "main.go",
			StartLine:   1,
			EndLine:     1,
		},
	}

	result, err := validator.ValidateWithContext(
		context.Background(),
		request,
		reviewContext,
		findings,
	)
	if err != nil {
		t.Fatalf("ValidateWithContext returned error: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf(
			"expected unchanged-line finding to be removed, got %d findings",
			len(result),
		)
	}
}

func TestFindingValidator_ValidateWithContext_RemovesDuplicateFindings(t *testing.T) {
	validator := &FindingValidator{}

	request := review.ReviewRequest{
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 1,
	}

	reviewContext := review.ReviewContext{
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "main.go",
				Status: "modified",
				Patch:  "@@ -1,2 +1,3 @@\n package main\n+\n func main() {}\n",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
	}

	findings := []review.ReviewFinding{
		{
			FindingID:   "duplicate-1",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.95,
			Title:       "Potential issue",
			Explanation: "The same issue was detected.",
			Suggestion:  "Fix the issue.",
			FilePath:    "main.go",
			StartLine:   3,
			EndLine:     3,
		},
		{
			FindingID:   "duplicate-2",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.90,
			Title:       "Potential issue",
			Explanation: "The same issue was detected.",
			Suggestion:  "Fix the issue.",
			FilePath:    "main.go",
			StartLine:   3,
			EndLine:     3,
		},
	}

	result, err := validator.ValidateWithContext(
		context.Background(),
		request,
		reviewContext,
		findings,
	)
	if err != nil {
		t.Fatalf("ValidateWithContext returned error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf(
			"expected 1 finding after deduplication, got %d",
			len(result),
		)
	}
}

func TestFindingValidator_ValidateWithContext_AllowsAnalyzerAndModelFindings(t *testing.T) {
	validator := &FindingValidator{}

	request := review.ReviewRequest{
		Organization:      "MorningBlossom",
		Repository:        "test-repo",
		PullRequestNumber: 1,
	}

	reviewContext := review.ReviewContext{
		ChangedFiles: []review.ChangedFile{
			{
				Path:   "main.go",
				Status: "modified",
				Patch:  "@@ -1,3 +1,4 @@\n package main\n+\n func main() {}\n",
			},
		},
		SourceFiles: []review.SourceFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
		AnalyzerFindings: []review.ReviewFinding{
			{
				FindingID:   "gofmt:main.go",
				Source:      "gofmt",
				Category:    "style",
				Severity:    "low",
				Confidence:  1.0,
				Title:       "File is not gofmt formatted",
				Explanation: "The file is not formatted according to gofmt.",
				Suggestion:  "Run gofmt on the file.",
				FilePath:    "main.go",
			},
		},
	}

	findings := []review.ReviewFinding{
		{
			FindingID:   "analyzer-finding",
			Source:      "gofmt",
			Category:    "style",
			Severity:    "low",
			Confidence:  1.0,
			Title:       "File is not gofmt formatted",
			Explanation: "The file is not formatted according to gofmt.",
			Suggestion:  "Run gofmt on the file.",
			FilePath:    "main.go",
			StartLine:   3,
			EndLine:     3,
		},
		{
			FindingID:   "model-finding",
			Source:      "model",
			Category:    "correctness",
			Severity:    "medium",
			Confidence:  0.9,
			Title:       "Potential correctness issue",
			Explanation: "The implementation may contain a correctness problem.",
			Suggestion:  "Review the implementation.",
			FilePath:    "main.go",
			StartLine:   2,
			EndLine:     2,
		},
	}

	result, err := validator.ValidateWithContext(
		context.Background(),
		request,
		reviewContext,
		findings,
	)
	if err != nil {
		t.Fatalf("ValidateWithContext returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf(
			"expected analyzer and model findings to be retained, got %d findings",
			len(result),
		)
	}
}
