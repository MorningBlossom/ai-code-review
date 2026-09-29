package model

import "testing"

func TestParseFindings(t *testing.T) {
	content := `{
		"findings": [
			{
				"category": "security",
				"severity": "high",
				"confidence": 0.95,
				"title": "Potential hardcoded credential",
				"explanation": "A credential appears to be embedded in source code.",
				"suggestion": "Move the credential to a secure secret store.",
				"file_path": "config.go",
				"start_line": 42,
				"end_line": 42,
				"evidence": "password := \"secret\""
			}
		]
	}`

	findings, err := parseFindings(content)
	if err != nil {
		t.Fatalf(
			"expected no error, got %v",
			err,
		)
	}

	if len(findings) != 1 {
		t.Fatalf(
			"expected one finding, got %d",
			len(findings),
		)
	}

	finding := findings[0]

	if finding.Source != "local-model" {
		t.Fatalf(
			"expected source local-model, got %q",
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

	if finding.FilePath != "config.go" {
		t.Fatalf(
			"expected file config.go, got %q",
			finding.FilePath,
		)

	}

	if finding.StartLine != 42 {
		t.Fatalf(
			"expected line 42, got %d",
			finding.StartLine,
		)
	}
}

func TestParseFindings_InvalidJSON(t *testing.T) {
	_, err := parseFindings(
		`{"findings": [}`,
	)

	if err == nil {
		t.Fatal("expected parsing error")
	}
}

func TestParseFindings_MarkdownFence(t *testing.T) {
	content := "```json\n" +
		`{"findings":[]}` +
		"\n```"

	findings, err := parseFindings(content)

	if err != nil {
		t.Fatalf(
			"expected no error, got %v",
			err,
		)
	}

	if len(findings) != 0 {
		t.Fatalf(
			"expected no findings, got %d",
			len(findings),
		)
	}
}
