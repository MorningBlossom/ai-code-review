package analyzer

import (
	"context"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestGoRaceAnalyzer_Pass(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
			Stdout:   "ok",
		},
	}

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "passed" {
		t.Fatalf("expected status passed, got %q", result.Status)
	}

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}

	if len(result.Diagnostics) != 0 {
		t.Fatalf(
			"expected no diagnostics, got %d",
			len(result.Diagnostics),
		)
	}
}

func TestGoRaceAnalyzer_RaceDetected(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr: `WARNING: DATA RACE
Read at 0x00c000 by goroutine 8:
  example.com/service.TestRace()
      service_test.go:42 +0x123
Previous write at 0x00c000 by goroutine 7:
  example.com/service.TestRace()
      service_test.go:35 +0x101`,
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

	if finding.Source != "go-race" {
		t.Errorf("expected source go-race, got %q", finding.Source)
	}

	if finding.Category != "concurrency" {
		t.Errorf(
			"expected category concurrency, got %q",
			finding.Category,
		)
	}

	if finding.Severity != "high" {
		t.Errorf(
			"expected severity high, got %q",
			finding.Severity,
		)
	}

	if finding.Confidence != 1 {
		t.Errorf(
			"expected confidence 1, got %v",
			finding.Confidence,
		)
	}

	if finding.Title != "Go race detector reported a data race" {
		t.Errorf(
			"unexpected title: %q",
			finding.Title,
		)
	}

	if !strings.Contains(
		finding.Explanation,
		"potential data race",
	) {
		t.Errorf(
			"unexpected explanation: %q",
			finding.Explanation,
		)
	}

	if !strings.Contains(
		finding.Suggestion,
		"synchronization",
	) {
		t.Errorf(
			"unexpected suggestion: %q",
			finding.Suggestion,
		)
	}

	if !strings.Contains(
		finding.Evidence,
		"WARNING: DATA RACE",
	) {
		t.Errorf(
			"expected race output in evidence, got %q",
			finding.Evidence,
		)
	}
}

func TestGoRaceAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "WARNING: DATA RACE\nrace detected",
		},
	}

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	if !strings.Contains(
		result.Findings[0].Evidence,
		"WARNING: DATA RACE",
	) {
		t.Errorf(
			"expected stdout to be used as evidence, got %q",
			result.Findings[0].Evidence,
		)
	}
}

func TestGoRaceAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: -1,
			Stderr:   "go: command not found",
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
		"go race execution failed",
	) {
		t.Errorf(
			"unexpected diagnostic: %q",
			result.Diagnostics[0],
		)
	}
}

func TestGoRaceAnalyzer_OrdinaryTestFailureDoesNotCreateRaceFinding(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr: `--- FAIL: TestSomething (0.01s)
expected 1, got 2
FAIL`,
		},
	}

	result := analyzer.AnalyzeWorkspace(context.Background(), ws)

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected no race findings for ordinary test failure, got %d",
			len(result.Findings),
		)
	}

	if len(result.Diagnostics) != 1 {
		t.Fatalf(
			"expected 1 diagnostic, got %d",
			len(result.Diagnostics),
		)
	}
}
