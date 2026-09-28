package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type StaticcheckAnalyzer struct{}

func NewStaticcheckAnalyzer() *StaticcheckAnalyzer {
	return &StaticcheckAnalyzer{}
}

func (a *StaticcheckAnalyzer) Name() string {
	return "staticcheck"
}

func (a *StaticcheckAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "skipped",
		Diagnostics: []string{
			"staticcheck requires a repository workspace",
		},
	}
}

func (a *StaticcheckAnalyzer) AnalyzeWorkspace(
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
		"staticcheck",
		"./...",
	)

	result.DurationMillis = time.Since(start).Milliseconds()

	if commandResult.ExitCode == -1 {
		result.Status = "failed"

		if strings.TrimSpace(commandResult.Stderr) != "" {
			result.Diagnostics = append(
				result.Diagnostics,
				fmt.Sprintf(
					"staticcheck execution failed: %s",
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
		parseStaticcheckFindings(output)...,
	)

	return result
}

func parseStaticcheckFindings(
	output string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		filePath, lineNumber, message, ok := parseStaticcheckDiagnostic(line)
		if !ok {
			continue
		}

		findings = append(
			findings,
			review.ReviewFinding{
				FindingID: fmt.Sprintf(
					"staticcheck:%s:%d:%s",
					filePath,
					lineNumber,
					message,
				),
				Source:      "staticcheck",
				Category:    "correctness",
				Severity:    "medium",
				Confidence:  1.0,
				Title:       "Staticcheck reported an issue",
				Explanation: message,
				FilePath:    filePath,
				StartLine:   lineNumber,
				EndLine:     lineNumber,
				Evidence:    line,
			},
		)
	}

	return findings
}

func parseStaticcheckDiagnostic(
	line string,
) (string, int, string, bool) {
	firstColon := strings.Index(line, ":")
	if firstColon <= 0 {
		return "", 0, "", false
	}

	filePath := line[:firstColon]

	remaining := line[firstColon+1:]

	secondColon := strings.Index(remaining, ":")
	if secondColon <= 0 {
		return "", 0, "", false
	}

	lineNumberText := remaining[:secondColon]

	var lineNumber int
	if _, err := fmt.Sscanf(
		lineNumberText,
		"%d",
		&lineNumber,
	); err != nil {
		return "", 0, "", false
	}

	remaining = remaining[secondColon+1:]

	// Staticcheck output normally looks like:
	//
	// file.go:42:3: message
	//
	// Remove the column number.
	if columnSeparator := strings.Index(remaining, ":"); columnSeparator >= 0 {
		columnText := strings.TrimSpace(
			remaining[:columnSeparator],
		)

		var column int
		if _, err := fmt.Sscanf(columnText, "%d", &column); err == nil {
			remaining = remaining[columnSeparator+1:]
		}
	}

	message := strings.TrimSpace(remaining)

	if message == "" {
		return "", 0, "", false
	}

	return filePath, lineNumber, message, true
}
