package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type GovulncheckAnalyzer struct{}

func NewGovulncheckAnalyzer() *GovulncheckAnalyzer {
	return &GovulncheckAnalyzer{}
}

func (a *GovulncheckAnalyzer) Name() string {
	return "govulncheck"
}

func (a *GovulncheckAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	return review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "skipped",
		Diagnostics: []string{
			"govulncheck requires a repository workspace",
		},
	}
}

func (a *GovulncheckAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	result := review.AnalyzerResult{
		AnalyzerName: a.Name(),
		Status:       "running",
	}

	commandResult := ws.Run(
		ctx,
		"govulncheck",
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
				"govulncheck execution failed: %s",
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

	result.Findings = parseGovulncheckFindings(output)

	return result
}

func parseGovulncheckFindings(output string) []review.ReviewFinding {
	lines := strings.Split(output, "\n")

	var findings []review.ReviewFinding

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		filePath, lineNumber, message, ok := parseGovulncheckDiagnostic(line)
		if !ok {
			continue
		}

		findings = append(
			findings,
			review.ReviewFinding{
				Source:      "govulncheck",
				Category:    "security",
				Severity:    "high",
				Confidence:  1,
				Title:       "Govulncheck reported a vulnerability",
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

func parseGovulncheckDiagnostic(
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

	message := strings.TrimSpace(
		remaining[secondColon+1:],
	)

	if message == "" {
		return "", 0, "", false
	}

	return filePath, lineNumber, message, true
}
