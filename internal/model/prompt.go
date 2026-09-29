package model

import (
	"fmt"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

func buildReviewPrompt(
	ctx review.ReviewContext,
) string {
	var b strings.Builder

	b.WriteString("Review the following pull request.\n\n")

	b.WriteString("Pull Request:\n")
	fmt.Fprintf(&b, "Title: %s\n", ctx.PullRequestTitle)
	fmt.Fprintf(&b, "Description: %s\n\n", ctx.PullRequestBody)

	b.WriteString("Changed Files:\n")

	for _, file := range ctx.ChangedFiles {
		fmt.Fprintf(
			&b,
			"- %s (%s, +%d/-%d)\n",
			file.Path,
			file.Status,
			file.Additions,
			file.Deletions,
		)

		if file.Patch != "" {
			b.WriteString("Patch:\n")
			b.WriteString(file.Patch)
			b.WriteString("\n")
		}
	}

	b.WriteString("\nSource Files:\n")

	for _, file := range ctx.SourceFiles {
		fmt.Fprintf(
			&b,
			"\n--- %s ---\n",
			file.Path,
		)

		b.WriteString(file.Content)
		b.WriteString("\n")
	}

	if len(ctx.AnalyzerFindings) > 0 {
		b.WriteString("\nDeterministic Analyzer Findings:\n")

		for _, finding := range ctx.AnalyzerFindings {
			fmt.Fprintf(
				&b,
				"- [%s] %s: %s (%s:%d)\n",
				finding.Severity,
				finding.Title,
				finding.Explanation,
				finding.FilePath,
				finding.StartLine,
			)
		}
	}

	b.WriteString(`
Return findings using this JSON structure:

{
  "findings": [
    {
      "category": "correctness|security|concurrency|reliability|maintainability|test",
      "severity": "critical|high|medium|low",
      "confidence": 0.0,
      "title": "short title",
      "explanation": "specific explanation",
      "suggestion": "specific remediation",
      "file_path": "path/to/file.go",
      "start_line": 10,
      "end_line": 10,
      "evidence": "specific evidence from the supplied code"
    }
  ]
}

Rules:
- Only report issues supported by the supplied code.
- file_path must refer to a supplied file.
- Line numbers must refer to the supplied source.
- Prefer changed code.
- Do not invent files or line numbers.
- Do not report formatting-only issues.
- Do not include markdown fences.
- Return only JSON.
`)

	return b.String()
}
