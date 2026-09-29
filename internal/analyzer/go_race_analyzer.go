package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type GoRaceAnalyzer struct{}

func NewGoRaceAnalyzer() *GoRaceAnalyzer {
	return &GoRaceAnalyzer{}
}

func (a *GoRaceAnalyzer) Name() string {
	return "go-race"
}

func (a *GoRaceAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "skipped",
		Diagnostics: []string{
			"go race detection requires a repository workspace",
		},
	}
}

func (a *GoRaceAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	result := review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "running",
	}

	commandResult := ws.Run(
		ctx,
		"go",
		"test",
		"-race",
		"./...",
	)

	output := strings.TrimSpace(commandResult.Stderr)

	if output == "" {
		output = strings.TrimSpace(commandResult.Stdout)
	}

	if commandResult.ExitCode == 0 {
		result.Status = "passed"
		return result
	}

	if commandResult.ExitCode == -1 {
		result.Status = "failed"
		result.Diagnostics = append(
			result.Diagnostics,
			fmt.Sprintf(
				"go race execution failed: %s",
				output,
			),
		)

		return result
	}

	result.Status = "failed"

	if output != "" {
		result.Diagnostics = append(
			result.Diagnostics,
			output,
		)
	}

	result.Findings = []review.ReviewFinding{
		{
			Source:      "go-race",
			Category:    "concurrency",
			Severity:    "high",
			Confidence:  1,
			Title:       "Go race detector reported a data race",
			Explanation: "The Go race detector found a potential data race while running the test suite.",
			Suggestion:  "Protect shared state with appropriate synchronization such as mutexes, channels, or atomic operations.",
			Evidence:    output,
		},
	}

	return result
}
