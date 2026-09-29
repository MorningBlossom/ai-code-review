package analyzer

import (
	"context"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type fakeCommandRunner struct {
	results map[string]CommandResult
}

func (f *fakeCommandRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) CommandResult {
	key := name

	for _, arg := range args {
		key += " " + arg
	}

	if result, ok := f.results[key]; ok {
		return result
	}

	return CommandResult{
		ExitCode: 0,
	}
}

func (f *fakeCommandRunner) RunWithInput(
	ctx context.Context,
	input string,
	name string,
	args ...string,
) CommandResult {
	key := name

	for _, arg := range args {
		key += " " + arg
	}

	if result, ok := f.results[key]; ok {
		return result
	}

	return CommandResult{
		ExitCode: 0,
	}
}

func TestGofmtAnalyzer_Passed(t *testing.T) {
	runner := &fakeCommandRunner{
		results: map[string]CommandResult{
			"gofmt -d": {
				ExitCode: 0,
				Stdout:   "",
			},
		},
	}

	gofmtAnalyzer := NewGofmtAnalyzer(runner)

	reviewContext := review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "payment/retry.go",
				Content: "package payment\n",
			},
		},
	}

	result := gofmtAnalyzer.Analyze(
		context.Background(),
		reviewContext,
	)

	if result.Status != "passed" {
		t.Fatalf(
			"expected status passed, got %q",
			result.Status,
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected 0 findings, got %d",
			len(result.Findings),
		)
	}
}

func TestGofmtAnalyzer_FindsFormattingIssue(t *testing.T) {
	runner := &fakeCommandRunner{
		results: map[string]CommandResult{
			"gofmt -d": {
				ExitCode: 1,
				Stdout: `diff
--- a/payment/retry.go
+++ b/payment/retry.go
@@
-func retryPayment( ) error {
+func retryPayment() error {
`,
			},
		},
	}

	gofmtAnalyzer := NewGofmtAnalyzer(runner)

	reviewContext := review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path: "payment/retry.go",
				Content: `package payment

func retryPayment( ) error {
	return nil
}
`,
			},
		},
	}

	result := gofmtAnalyzer.Analyze(
		context.Background(),
		reviewContext,
	)

	if result.Status != "passed" {
		t.Fatalf(
			"expected status passed, got %q",
			result.Status,
		)
	}

	if len(result.Findings) != 1 {
		t.Fatalf(
			"expected 1 finding, got %d",
			len(result.Findings),
		)
	}

	finding := result.Findings[0]

	if finding.Source != "gofmt" {
		t.Fatalf(
			"expected source gofmt, got %q",
			finding.Source,
		)
	}

	if finding.Category != "style" {
		t.Fatalf(
			"expected category style, got %q",
			finding.Category,
		)
	}

	if finding.Severity != "low" {
		t.Fatalf(
			"expected severity low, got %q",
			finding.Severity,
		)
	}

	if finding.FilePath != "payment/retry.go" {
		t.Fatalf(
			"expected file payment/retry.go, got %q",
			finding.FilePath,
		)
	}

	if finding.Confidence != 1.0 {
		t.Fatalf(
			"expected confidence 1.0, got %f",
			finding.Confidence,
		)
	}

	if finding.Evidence == "" {
		t.Fatal("expected finding evidence")
	}
}

func TestGofmtAnalyzer_IgnoresNonGoFiles(t *testing.T) {
	runner := &fakeCommandRunner{
		results: map[string]CommandResult{},
	}

	gofmtAnalyzer := NewGofmtAnalyzer(runner)

	reviewContext := review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "README.md",
				Content: "# Test",
			},
			{
				Path:    "config.yaml",
				Content: "enabled: true",
			},
		},
	}

	result := gofmtAnalyzer.Analyze(
		context.Background(),
		reviewContext,
	)

	if result.Status != "passed" {
		t.Fatalf(
			"expected status passed, got %q",
			result.Status,
		)
	}

	if len(result.Findings) != 0 {
		t.Fatalf(
			"expected 0 findings, got %d",
			len(result.Findings),
		)
	}
}

func TestGofmtAnalyzer_CommandFailure(t *testing.T) {
	runner := &fakeCommandRunner{
		results: map[string]CommandResult{
			"gofmt -d": {
				ExitCode: -1,
				Stderr:   "gofmt: command failed",
			},
		},
	}

	gofmtAnalyzer := NewGofmtAnalyzer(runner)

	reviewContext := review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "payment/retry.go",
				Content: "package payment\n",
			},
		},
	}

	result := gofmtAnalyzer.Analyze(
		context.Background(),
		reviewContext,
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
}
