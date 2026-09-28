package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestStaticcheckAnalyzer_Passed(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "staticcheck" {
		t.Fatalf(
			"expected analyzer name staticcheck, got %q",
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

func TestStaticcheckAnalyzer_FindsDiagnostic(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr:   "payment/retry.go:42:3: should use strings.Contains instead",
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

	if finding.Source != "staticcheck" {
		t.Fatalf(
			"expected source staticcheck, got %q",
			finding.Source,
		)
	}

	if finding.FilePath != "payment/retry.go" {
		t.Fatalf(
			"expected file payment/retry.go, got %q",
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

	if finding.Explanation != "should use strings.Contains instead" {
		t.Fatalf(
			"unexpected explanation: %q",
			finding.Explanation,
		)
	}
}

func TestStaticcheckAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "payment/retry.go:42:3: should use strings.Contains instead",
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

func TestStaticcheckAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: -1,
			Stderr:   "staticcheck: command not found",
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

func TestParseStaticcheckDiagnostic(t *testing.T) {
	filePath, lineNumber, message, ok := parseStaticcheckDiagnostic(
		"payment/retry.go:42:3: should use strings.Contains instead",
	)

	if !ok {
		t.Fatal("expected diagnostic to parse")
	}

	if filePath != "payment/retry.go" {
		t.Fatalf(
			"expected file path payment/retry.go, got %q",
			filePath,
		)
	}

	if lineNumber != 42 {
		t.Fatalf(
			"expected line 42, got %d",
			lineNumber,
		)
	}

	if message != "should use strings.Contains instead" {
		t.Fatalf(
			"unexpected message: %q",
			message,
		)
	}
}
