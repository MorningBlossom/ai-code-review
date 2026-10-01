package analyzer

import (
	"context"
	"strings"
	"testing"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func TestSecurityRulesAnalyzer_InsecureTLS(t *testing.T) {
	source := `package example

import "crypto/tls"

func insecure() {
	_ = &tls.Config{
		InsecureSkipVerify: true,
	}
}
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "client.go",
				Content: source,
			},
		},
	})

	if result.Status != "passed" {
		t.Fatalf("expected status passed, got %q", result.Status)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	finding := result.Findings[0]

	if finding.FindingID != "SEC001" {
		t.Fatalf("expected SEC001, got %q", finding.FindingID)
	}

	if finding.Severity != "high" {
		t.Fatalf("expected high severity, got %q", finding.Severity)
	}

	if finding.Category != "security" {
		t.Fatalf("expected security category, got %q", finding.Category)
	}
}

func TestSecurityRulesAnalyzer_SafeTLS(t *testing.T) {
	source := `package example

import "crypto/tls"

func secure() {
	_ = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
}
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "client.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestSecurityRulesAnalyzer_DynamicCommand(t *testing.T) {
	source := `package example

import (
	"os/exec"
)

func run(command string) {
	_ = exec.Command(command)
}
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "command.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	if result.Findings[0].FindingID != "SEC002" {
		t.Fatalf(
			"expected SEC002, got %q",
			result.Findings[0].FindingID,
		)
	}
}

func TestSecurityRulesAnalyzer_StaticCommand(t *testing.T) {
	source := `package example

import (
	"os/exec"
)

func run() {
	_ = exec.Command("git", "status")
}
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "command.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestSecurityRulesAnalyzer_WeakRandom(t *testing.T) {
	source := `package example

import "math/rand"

func generate() int {
	return rand.Int()
}
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "random.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	if result.Findings[0].FindingID != "SEC003" {
		t.Fatalf(
			"expected SEC003, got %q",
			result.Findings[0].FindingID,
		)
	}
}

func TestSecurityRulesAnalyzer_HardcodedSecret(t *testing.T) {
	source := `package example

const config = "password=super-secret-password"
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "config.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	if result.Findings[0].FindingID != "SEC004" {
		t.Fatalf(
			"expected SEC004, got %q",
			result.Findings[0].FindingID,
		)
	}
}

func TestSecurityRulesAnalyzer_IgnoresPlaceholderSecret(t *testing.T) {
	source := `package example

const config = "password=changeme"
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "config.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestSecurityRulesAnalyzer_IgnoresNonGoFiles(t *testing.T) {
	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "README.md",
				Content: `password=super-secret-password`,
			},
			{
				Path:    "config.yaml",
				Content: `password: super-secret-password`,
			},
		},
	})

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestSecurityRulesAnalyzer_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(ctx, review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "client.go",
				Content: `package example`,
			},
		},
	})

	if result.Status != "failed" {
		t.Fatalf("expected status failed, got %q", result.Status)
	}

	if len(result.Diagnostics) == 0 {
		t.Fatal("expected cancellation diagnostic")
	}
}

func TestSecurityRulesAnalyzer_FindingMetadata(t *testing.T) {
	source := `package example

import "crypto/tls"

func insecure() {
	_ = &tls.Config{
		InsecureSkipVerify: true,
	}
}
`

	analyzer := NewSecurityRulesAnalyzer()

	result := analyzer.Analyze(context.Background(), review.ReviewContext{
		SourceFiles: []review.SourceFile{
			{
				Path:    "client.go",
				Content: source,
			},
		},
	})

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	finding := result.Findings[0]

	if finding.FindingID == "" {
		t.Fatal("finding ID must not be empty")
	}

	if finding.Source != "security-rules" {
		t.Fatalf("expected source security-rules, got %q", finding.Source)
	}

	if finding.Title == "" {
		t.Fatal("finding title must not be empty")
	}

	if finding.Explanation == "" {
		t.Fatal("finding explanation must not be empty")
	}

	if finding.Suggestion == "" {
		t.Fatal("finding suggestion must not be empty")
	}

	if finding.FilePath != "client.go" {
		t.Fatalf("expected client.go, got %q", finding.FilePath)
	}

	if finding.StartLine <= 0 {
		t.Fatalf("expected positive start line, got %d", finding.StartLine)
	}

	if finding.EndLine < finding.StartLine {
		t.Fatalf(
			"expected end line >= start line, got %d-%d",
			finding.StartLine,
			finding.EndLine,
		)
	}

	if finding.Confidence < 0 || finding.Confidence > 1 {
		t.Fatalf(
			"confidence must be between 0 and 1, got %f",
			finding.Confidence,
		)
	}

	if !strings.Contains(finding.Title, "TLS") {
		t.Fatalf(
			"expected TLS-related title, got %q",
			finding.Title,
		)
	}
}
