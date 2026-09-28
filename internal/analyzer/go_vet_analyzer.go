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

type GoVetAnalyzer struct {
}

func NewGoVetAnalyzer() *GoVetAnalyzer {
	return &GoVetAnalyzer{}
}

func (a *GoVetAnalyzer) Name() string {
	return "go-vet"
}

func (a *GoVetAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	// Repository-level analyzers require a workspace.
	// The workspace is attached separately through AnalyzeWorkspace.
	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "skipped",
		Diagnostics: []string{
			"go vet requires a repository workspace",
		},
	}
}
func (a *GoVetAnalyzer) AnalyzeWorkspace(
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
		"vet",
		"./...",
	)

	result.DurationMillis = time.Since(start).Milliseconds()

	if commandResult.ExitCode == -1 {
		result.Status = "failed"
		result.Diagnostics = append(
			result.Diagnostics,
			fmt.Sprintf(
				"go vet execution failed: %s",
				commandResult.Stderr,
			),
		)
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
		parseGoVetFindings(output)...,
	)

	return result
}

func parseGoVetFindings(output string) []review.ReviewFinding {
	var findings []review.ReviewFinding

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		filePath, lineNumber, message, ok := parseGoVetDiagnostic(line)
		if !ok {
			continue
		}

		findings = append(
			findings,
			review.ReviewFinding{
				FindingID: fmt.Sprintf(
					"go-vet:%s:%d:%s",
					filePath,
					lineNumber,
					message,
				),
				Source:      "go-vet",
				Category:    "correctness",
				Severity:    "medium",
				Confidence:  1.0,
				Title:       "go vet reported an issue",
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
func parseGoVetDiagnostic(
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

	lineNumber, err := strconv.Atoi(lineNumberText)
	if err != nil {
		return "", 0, "", false
	}

	remaining = remaining[secondColon+1:]

	if columnSeparator := strings.Index(remaining, ":"); columnSeparator >= 0 {
		if _, err := strconv.Atoi(
			strings.TrimSpace(remaining[:columnSeparator]),
		); err == nil {
			remaining = remaining[columnSeparator+1:]
		}
	}

	message := strings.TrimSpace(remaining)

	if message == "" {
		return "", 0, "", false
	}

	return filePath, lineNumber, message, true
}
