package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type GoTestAnalyzer struct{}

func NewGoTestAnalyzer() *GoTestAnalyzer {
	return &GoTestAnalyzer{}
}

func (a *GoTestAnalyzer) Name() string {
	return "go-test"
}

func (a *GoTestAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "skipped",
		Diagnostics: []string{
			"go test requires a repository workspace",
		},
	}
}

func (a *GoTestAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	start := time.Now()

	result := review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "passed",
	}

	commandResult := ws.Run(
		ctx,
		"go",
		"test",
		"./...",
	)

	result.DurationMillis = time.Since(start).Milliseconds()

	if commandResult.ExitCode == -1 {
		result.Status = "failed"

		if strings.TrimSpace(commandResult.Stderr) != "" {
			result.Diagnostics = append(
				result.Diagnostics,
				fmt.Sprintf(
					"go test execution failed: %s",
					strings.TrimSpace(commandResult.Stderr),
				),
			)
		}

		return result
	}

	if commandResult.ExitCode == 0 {
		return result
	}

	result.Status = "failed"

	output := strings.TrimSpace(commandResult.Stderr)
	if output == "" {
		output = strings.TrimSpace(commandResult.Stdout)
	}

	if output == "" {
		return result
	}

	result.Diagnostics = append(
		result.Diagnostics,
		output,
	)

	result.Findings = append(
		result.Findings,
		review.ReviewFinding{
			FindingID:   fmt.Sprintf("go-test:%s", "repository"),
			Source:      a.Name(),
			Category:    "test",
			Severity:    "high",
			Confidence:  1.0,
			Title:       "Go tests are failing",
			Explanation: "The repository test suite failed when running go test ./....",
			Suggestion:  "Fix the failing tests before merging the pull request.",
			Evidence:    output,
		},
	)

	return result
}
