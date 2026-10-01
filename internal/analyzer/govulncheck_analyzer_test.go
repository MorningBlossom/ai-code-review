package analyzer

import (
	"context"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestGovulncheckAnalyzer_Pass(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
			Stdout:   "govulncheck: no vulnerabilities found",
		},
	}

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "passed" {
		t.Fatalf("expected status passed, got %q", result.Status)
	}

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestGovulncheckAnalyzer_Vulnerability(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr:   `main.go:42:2: golang.org/x/crypto vulnerable to CVE-2025-12345`,
		},
	}

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	finding := result.Findings[0]

	if finding.Source != "govulncheck" {
		t.Errorf("expected source govulncheck, got %q", finding.Source)
	}

	if finding.Category != "security" {
		t.Errorf("expected category security, got %q", finding.Category)
	}

	if finding.Severity != "high" {
		t.Errorf("expected severity high, got %q", finding.Severity)
	}

	if finding.FilePath != "main.go" {
		t.Errorf("expected file path main.go, got %q", finding.FilePath)
	}

	if finding.StartLine != 42 {
		t.Errorf("expected start line 42, got %d", finding.StartLine)
	}

	if finding.EndLine != 42 {
		t.Errorf("expected end line 42, got %d", finding.EndLine)
	}

	if !strings.Contains(finding.Explanation, "CVE-2025-12345") {
		t.Errorf(
			"expected explanation to contain vulnerability, got %q",
			finding.Explanation,
		)
	}
}

func TestGovulncheckAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewGovulncheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   `internal/api/handler.go:18:5: vulnerable dependency detected`,
		},
	}

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	if result.Findings[0].FilePath != "internal/api/handler.go" {
		t.Errorf(
			"unexpected file path: %q",
			result.Findings[0].FilePath,
		)
	}

	if result.Findings[0].StartLine != 18 {
		t.Errorf(
			"unexpected line: %d",
			result.Findings[0].StartLine,
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

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no findings on execution failure, got %d",
			len(result.Findings),
		)
	}

	if len(result.Diagnostics) != 1 {
		t.Fatalf(
			"expected 1 diagnostic, got %d",
			len(result.Diagnostics),
		)
	}

	if !strings.Contains(
		result.Diagnostics[0],
		"govulncheck execution failed",
	) {
		t.Errorf(
			"unexpected diagnostic: %q",
			result.Diagnostics[0],
		)
	}
}
