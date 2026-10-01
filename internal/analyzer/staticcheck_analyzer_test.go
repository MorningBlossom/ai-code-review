package analyzer

import (
	"context"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestStaticcheckAnalyzer_Pass(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
			Stdout:   "staticcheck: no issues",
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

func TestStaticcheckAnalyzer_Finding(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr:   "internal/service/service.go:42:5: this value is never used",
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

	if finding.Source != "staticcheck" {
		t.Errorf("expected source staticcheck, got %q", finding.Source)
	}

	if finding.Category != "correctness" {
		t.Errorf("expected category correctness, got %q", finding.Category)
	}

	if finding.Severity != "medium" {
		t.Errorf("expected severity medium, got %q", finding.Severity)
	}

	if finding.FilePath != "internal/service/service.go" {
		t.Errorf("unexpected file path: %q", finding.FilePath)
	}

	if finding.StartLine != 42 {
		t.Errorf("expected start line 42, got %d", finding.StartLine)
	}

	if finding.EndLine != 42 {
		t.Errorf("expected end line 42, got %d", finding.EndLine)
	}

	if !strings.Contains(
		finding.Explanation,
		"this value is never used",
	) {
		t.Errorf(
			"unexpected explanation: %q",
			finding.Explanation,
		)
	}

	if finding.FindingID == "" {
		t.Error("expected finding ID to be populated")
	}
}

func TestStaticcheckAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewStaticcheckAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "internal/api/handler.go:18:5: error return value is ignored",
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

	if finding.FilePath != "internal/api/handler.go" {
		t.Errorf("unexpected file path: %q", finding.FilePath)
	}

	if finding.StartLine != 18 {
		t.Errorf("expected line 18, got %d", finding.StartLine)
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
		"staticcheck execution failed",
	) {
		t.Errorf(
			"unexpected diagnostic: %q",
			result.Diagnostics[0],
		)
	}
}
