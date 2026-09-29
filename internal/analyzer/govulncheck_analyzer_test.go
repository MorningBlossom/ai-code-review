package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestGovulncheckAnalyzer_Passed(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "govulncheck" {
		t.Fatalf(
			"expected analyzer name govulncheck, got %q",
			result.AnalyzerName,
		)
	}

	if result.Status != "passed" {
		t.Fatalf(
			"expected status passed, got %q",
			result.Status,
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings, got %d",
			len(result.Findings),
		)
	}
}

func TestGovulncheckAnalyzer_FindsVulnerability(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 3,
			Stderr:   "internal/http/client.go:42:5: golang.org/x/net/http2 vulnerable function",
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
			"expected one diagnostic, got %d",
			len(result.Diagnostics),
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected one finding, got %d",
			len(result.Findings),
		)
	}

	finding := result.Findings[0]

	if finding.Source != "govulncheck" {
		t.Fatalf(
			"expected source govulncheck, got %q",
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

	if finding.FilePath != "internal/http/client.go" {
		t.Fatalf(
			"expected file internal/http/client.go, got %q",
			finding.FilePath,
		)
	}

	if finding.StartLine != 42 {
		t.Fatalf(
			"expected start line 42, got %d",
			finding.StartLine,
		)
	}

	if finding.EndLine != 42 {
		t.Fatalf(
			"expected end line 42, got %d",
			finding.EndLine,
		)
	}
}

func TestGovulncheckAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "internal/http/client.go:42:5: vulnerable function",
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if len(result.Diagnostics) != 1 {
		t.Fatalf(
			"expected one diagnostic, got %d",
			len(result.Diagnostics),
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected one finding, got %d",
			len(result.Findings),
		)
	}
}

func TestGovulncheckAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: -1,
			Stderr:   "govulncheck: command not found",
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
			"expected one diagnostic, got %d",
			len(result.Diagnostics),
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings on execution failure, got %d",
			len(result.Findings),
		)
	}
}

func TestParseGovulncheckDiagnostic(t *testing.T) {
	filePath, lineNumber, message, ok := parseGovulncheckDiagnostic(
		"internal/http/client.go:42:5: vulnerable function",
	)

	if !ok {
		t.Fatal("expected diagnostic to parse")
	}

	if filePath != "internal/http/client.go" {
		t.Fatalf(
			"expected file path internal/http/client.go, got %q",
			filePath,
		)
	}

	if lineNumber != 42 {
		t.Fatalf(
			"expected line 42, got %d",
			lineNumber,
		)
	}

	if message != "5: vulnerable function" {
		t.Fatalf(
			"expected message %q, got %q",
			"5: vulnerable function",
			message,
		)
	}
}

func TestParseGovulncheckDiagnostic_Invalid(t *testing.T) {
	_, _, _, ok := parseGovulncheckDiagnostic(
		"not a diagnostic",
	)

	if ok {
		t.Fatal("expected diagnostic parsing to fail")
	}
}
