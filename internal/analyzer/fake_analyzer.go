package analyzer

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type FakeAnalyzer struct {
	Fail bool
}

func (f *FakeAnalyzer) Name() string {
	return "fake-analyzer"
}

func (f *FakeAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	if f.Fail {
		return review.AnalyzerResult{
			AnalyzerName:    "fake-analyzer",
			AnalyzerVersion: "1.0.0",
			Status:          "failed",
			Diagnostics: []string{
				"fake analyzer failed",
			},
		}
	}

	return review.AnalyzerResult{
		AnalyzerName:    "fake-analyzer",
		AnalyzerVersion: "1.0.0",
		Status:          "completed",
		Findings: []review.ReviewFinding{
			{
				FindingID:   "analyzer-001",
				Source:      "fake-analyzer",
				Category:    "bug",
				Severity:    "medium",
				Confidence:  0.95,
				Title:       "Potential ignored error",
				Explanation: "The returned error should be checked.",
				Suggestion:  "Handle the returned error explicitly.",
				FilePath:    "payment/retry.go",
				StartLine:   4,
				EndLine:     4,
			},
		},
	}
}
