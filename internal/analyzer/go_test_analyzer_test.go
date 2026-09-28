package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestGoTestAnalyzer_Passed(t *testing.T) {
	analyzer := NewGoTestAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "go-test" {
		t.Fatalf(
			"expected analyzer name go-test, got %q",
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

func TestGoTestAnalyzer_FindsTestFailure(t *testing.T) {
	analyzer := NewGoTestAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr: `--- FAIL: TestPaymentRetry (0.00s)
    retry_test.go:42: expected retry count 3, got 2
FAIL
FAIL    example.com/payment	0.002s`,
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

	if finding.Source != "go-test" {
		t.Fatalf(
			"expected source go-test, got %q",
			finding.Source,
		)
	}

	if finding.Category != "test" {
		t.Fatalf(
			"expected category test, got %q",
			finding.Category,
		)
	}

	if finding.Severity != "high" {
		t.Fatalf(
			"expected severity high, got %q",
			finding.Severity,
		)
	}

	if finding.Confidence != 1.0 {
		t.Fatalf(
			"expected confidence 1.0, got %f",
			finding.Confidence,
		)
	}

	if finding.Title != "Go tests are failing" {
		t.Fatalf(
			"unexpected title: %q",
			finding.Title,
		)
	}

	if finding.Evidence == "" {
		t.Fatal("expected test failure evidence")
	}
}

func TestGoTestAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewGoTestAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout: `--- FAIL: TestPaymentRetry (0.00s)
FAIL`,
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

func TestGoTestAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewGoTestAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: -1,
			Stderr:   "go: command not found",
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
