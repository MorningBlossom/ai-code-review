package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type GofmtAnalyzer struct {
	runner CommandRunner
}

func NewGofmtAnalyzer(runner CommandRunner) *GofmtAnalyzer {
	return &GofmtAnalyzer{
		runner: runner,
	}
}

func (a *GofmtAnalyzer) Name() string {
	return "gofmt"
}

func (a *GofmtAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	result := review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "passed",
	}

	for _, sourceFile := range reviewContext.SourceFiles {
		if !strings.HasSuffix(sourceFile.Path, ".go") {
			continue
		}

		commandResult := a.runner.RunWithInput(
			ctx,
			sourceFile.Content,
			"gofmt",
			"-d",
		)

		if commandResult.ExitCode == -1 {
			result.Status = "failed"
			result.Diagnostics = append(
				result.Diagnostics,
				fmt.Sprintf(
					"gofmt execution failed for %s: %s",
					sourceFile.Path,
					commandResult.Stderr,
				),
			)
			continue
		}

		// gofmt -d returns exit code 1 when formatting differences
		// are found. That is an analyzer finding, not an execution failure.
		if commandResult.ExitCode == 1 &&
			strings.TrimSpace(commandResult.Stdout) != "" {
			result.Findings = append(
				result.Findings,
				review.ReviewFinding{
					FindingID:   fmt.Sprintf("gofmt:%s", sourceFile.Path),
					Source:      a.Name(),
					Category:    "style",
					Severity:    "low",
					Confidence:  1.0,
					Title:       "File is not gofmt formatted",
					Explanation: "The Go source file has formatting differences according to gofmt.",
					Suggestion:  "Run gofmt on the file before merging.",
					FilePath:    sourceFile.Path,
					Evidence:    commandResult.Stdout,
				},
			)

			continue
		}

		if commandResult.ExitCode != 0 {
			result.Status = "failed"
			result.Diagnostics = append(
				result.Diagnostics,
				fmt.Sprintf(
					"gofmt failed for %s: %s",
					sourceFile.Path,
					commandResult.Stderr,
				),
			)
			continue
		}
	}

	return result
}
