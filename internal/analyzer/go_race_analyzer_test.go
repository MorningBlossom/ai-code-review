package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

func TestGoRaceAnalyzer_Passed(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "go-race" {
		t.Fatalf(
			"expected analyzer name go-race, got %q",
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

func TestGoRaceAnalyzer_FindsRace(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr: `
==================
WARNING: DATA RACE
Read at 0x00c0001 by goroutine 8:
  main.process()
      worker.go:42
==================
`,
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

	if finding.Source != "go-race" {
		t.Fatalf(
			"expected source go-race, got %q",
			finding.Source,
		)
	}

	if finding.Category != "concurrency" {
		t.Fatalf(
			"expected category concurrency, got %q",
			finding.Category,
		)
	}

	if finding.Severity != "high" {
		t.Fatalf(
			"expected severity high, got %q",
			finding.Severity,
		)
	}

	if finding.Confidence != 1 {
		t.Fatalf(
			"expected confidence 1, got %f",
			finding.Confidence,
		)
	}
}

func TestGoRaceAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "WARNING: DATA RACE",
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

func TestGoRaceAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewGoRaceAnalyzer()

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
