package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type fakeWorkspace struct {
	result workspace.CommandResult
}

func (w *fakeWorkspace) Root() string {
	return "/tmp/test-workspace"
}

func (w *fakeWorkspace) Run(
	ctx context.Context,
	name string,
	args ...string,
) workspace.CommandResult {
	return w.result
}

func (w *fakeWorkspace) Close() error {
	return nil
}

func TestGoVetAnalyzer_Passed(t *testing.T) {
	analyzer := NewGoVetAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 0,
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "go-vet" {
		t.Fatalf(
			"expected analyzer name go-vet, got %q",
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

func TestGoVetAnalyzer_FindsDiagnostic(t *testing.T) {
	analyzer := NewGoVetAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stderr:   "payment/retry.go:42:3: unreachable code",
		},
	}

	result := analyzer.AnalyzeWorkspace(
		context.Background(),
		ws,
	)

	if result.AnalyzerName != "go-vet" {
		t.Fatalf(
			"expected analyzer name go-vet, got %q",
			result.AnalyzerName,
		)
	}

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

	if finding.Source != "go-vet" {
		t.Fatalf(
			"expected source go-vet, got %q",
			finding.Source,
		)
	}

	if finding.Category != "correctness" {
		t.Fatalf(
			"expected category correctness, got %q",
			finding.Category,
		)
	}

	if finding.Severity != "medium" {
		t.Fatalf(
			"expected severity medium, got %q",
			finding.Severity,
		)
	}

	if finding.Confidence != 1.0 {
		t.Fatalf(
			"expected confidence 1.0, got %f",
			finding.Confidence,
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

	if finding.Explanation != "unreachable code" {
		t.Fatalf(
			"unexpected explanation: %q",
			finding.Explanation,
		)
	}

	if finding.Evidence != "payment/retry.go:42:3: unreachable code" {
		t.Fatalf(
			"unexpected evidence: %q",
			finding.Evidence,
		)
	}
}

func TestGoVetAnalyzer_UsesStdoutWhenStderrEmpty(t *testing.T) {
	analyzer := NewGoVetAnalyzer()

	ws := &fakeWorkspace{
		result: workspace.CommandResult{
			ExitCode: 1,
			Stdout:   "payment/retry.go:42:3: unreachable code",
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

	if result.Findings[0].FilePath != "payment/retry.go" {
		t.Fatalf(
			"expected file payment/retry.go, got %q",
			result.Findings[0].FilePath,
		)
	}
}

func TestGoVetAnalyzer_ExecutionFailure(t *testing.T) {
	analyzer := NewGoVetAnalyzer()

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

func TestParseGoVetDiagnostic(t *testing.T) {
	filePath, lineNumber, message, ok := parseGoVetDiagnostic(
		"payment/retry.go:42:3: unreachable code",
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

	if message != "unreachable code" {
		t.Fatalf(
			"unexpected message: %q",
			message,
		)
	}
}

func TestParseGoVetDiagnostic_Invalid(t *testing.T) {
	tests := []string{
		"",
		"invalid diagnostic",
		":42:3: missing file",
		"file.go:not-a-line: message",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, _, _, ok := parseGoVetDiagnostic(input)

			if ok {
				t.Fatalf(
					"expected diagnostic %q to be invalid",
					input,
				)
			}
		})
	}
}
