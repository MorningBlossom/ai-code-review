package analyzer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type GosecAnalyzer struct{}

func NewGosecAnalyzer() *GosecAnalyzer {
	return &GosecAnalyzer{}
}

func (a *GosecAnalyzer) Name() string {
	return "gosec"
}

func (a *GosecAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "skipped",
		Diagnostics: []string{
			"gosec requires a repository workspace",
		},
	}
}

func (a *GosecAnalyzer) AnalyzeWorkspace(
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
		"gosec",
		"./...",
	)

	result.DurationMillis = time.Since(start).Milliseconds()

	if commandResult.ExitCode == -1 {
		result.Status = "failed"

		if strings.TrimSpace(commandResult.Stderr) != "" {
			result.Diagnostics = append(
				result.Diagnostics,
				fmt.Sprintf(
					"gosec execution failed: %s",
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

	output := strings.TrimSpace(commandResult.Stdout)
	if output == "" {
		output = strings.TrimSpace(commandResult.Stderr)
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
		parseGosecFindings(output)...,
	)

	return result
}

func parseGosecFindings(
	output string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		filePath, lineNumber, issue, ok := parseGosecDiagnostic(line)
		if !ok {
			continue
		}

		findings = append(
			findings,
			review.ReviewFinding{
				FindingID: fmt.Sprintf(
					"gosec:%s:%d:%s",
					filePath,
					lineNumber,
					issue,
				),
				Source:      "gosec",
				Category:    "security",
				Severity:    "high",
				Confidence:  1.0,
				Title:       "Gosec reported a security issue",
				Explanation: issue,
				FilePath:    filePath,
				StartLine:   lineNumber,
				EndLine:     lineNumber,
				Evidence:    line,
			},
		)
	}

	return findings
}

func parseGosecDiagnostic(
	line string,
) (string, int, string, bool) {
	// Example:
	//
	// [G101] Potential hardcoded credentials
	//     File: payment/config.go
	//     Line: 42
	//
	// gosec also emits lines such as:
	//
	// payment/config.go:42:3: [G101] Potential hardcoded credentials
	//
	// Handle the direct file:line format here.

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

	lineNumber, err := strconv.Atoi(
		strings.TrimSpace(lineNumberText),
	)
	if err != nil {
		return "", 0, "", false
	}

	remaining = remaining[secondColon+1:]

	// Remove the column number when present.
	if columnSeparator := strings.Index(remaining, ":"); columnSeparator >= 0 {
		columnText := strings.TrimSpace(
			remaining[:columnSeparator],
		)

		if _, err := strconv.Atoi(columnText); err == nil {
			remaining = remaining[columnSeparator+1:]
		}
	}

	issue := strings.TrimSpace(remaining)

	if issue == "" {
		return "", 0, "", false
	}

	return filePath, lineNumber, issue, true
}
