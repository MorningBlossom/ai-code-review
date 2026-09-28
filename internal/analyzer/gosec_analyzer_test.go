package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestGosecAnalyzer_Passed(t *testing.T) {
	analyzer := NewGosecAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "gosec" {
		t.Fatalf(
			"expected analyzer name gosec, got %q",
			result.AnalyzerName,
		)
	}

	if result.Status != "passed" {
		t.Fatalf(
			"expected status passed, got %q",
			result.Status,
		)
	}

	if len(result.Diagnostics) != 0 {
		t.Fatalf(
			"expected no diagnostics, got %v",
			result.Diagnostics,
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings, got %d",
			len(result.Findings),
		)
	}
}

func TestGosecAnalyzer_FindsDiagnostic(t *testing.T) {
	analyzer := NewGosecAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "payment/config.go:42:3: [G101] Potential hardcoded credentials",
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}

	if len(result.Diagnostics) != 1 {
		t.Fatalf(
			"expected 1 diagnostic, got %d",
			len(result.Diagnostics),
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected 1 finding, got %d",
			len(result.Findings),
		)
	}

	finding := result.Findings[0]

	if finding.Source != "gosec" {
		t.Fatalf(
			"expected source gosec, got %q",
			finding.Source,
		)
	}

	if finding.Category != "security" {
		t.Fatalf(
			"expected category security, got %q",
			finding.Category,
		)
	}

	if finding.Severity != "high" {
		t.Fatalf(
			"expected severity high, got %q",
			finding.Severity,
		)
	}

	if finding.FilePath != "payment/config.go" {
		t.Fatalf(
			"expected file payment/config.go, got %q",
			finding.FilePath,
		)
	}

	if finding.StartLine != 42 {
		t.Fatalf(
			"expected line 42, got %d",
			finding.StartLine,
		)
	}

	if finding.EndLine != 42 {
		t.Fatalf(
			"expected end line 42, got %d",
			finding.EndLine,
		)
	}

	if finding.Explanation != "[G101] Potential hardcoded credentials" {
		t.Fatalf(
			"unexpected explanation: %q",
			finding.Explanation,
		)
	}
}

func TestGosecAnalyzer_UsesStderrWhenStdoutEmpty(t *testing.T) {
	analyzer := NewGosecAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr:   "payment/config.go:42:3: [G101] Potential hardcoded credentials",
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected 1 finding, got %d",
			len(result.Findings),
		)
	}
}

func TestGosecAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewGosecAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: -1,
			Stderr:   "gosec: command not found",
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.Status != "failed" {
		t.Fatalf(
			"expected status failed, got %q",
			result.Status,
		)
	}

	if len(result.Diagnostics) != 1 {
		t.Fatalf(
			"expected 1 diagnostic, got %d",
			len(result.Diagnostics),
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings for execution failure, got %d",
			len(result.Findings),
		)
	}
}

func TestParseGosecDiagnostic(t *testing.T) {
	filePath, lineNumber, issue, ok := parseGosecDiagnostic(
		"payment/config.go:42:3: [G101] Potential hardcoded credentials",
	)

	if !ok {
		t.Fatal("expected diagnostic to parse")
	}

	if filePath != "payment/config.go" {
		t.Fatalf(
			"expected file path payment/config.go, got %q",
			filePath,
		)
	}

	if lineNumber != 42 {
		t.Fatalf(
			"expected line 42, got %d",
			lineNumber,
		)
	}

	if issue != "[G101] Potential hardcoded credentials" {
		t.Fatalf(
			"unexpected issue: %q",
			issue,
		)
	}
}
