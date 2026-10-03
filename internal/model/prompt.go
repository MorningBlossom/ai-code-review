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
				"\n--- %s ---\n",
				finding.Source,
			)

			fmt.Fprintf(
				&b,
				"Severity: %s\n",
				finding.Severity,
			)

			fmt.Fprintf(
				&b,
				"Confidence: %.2f\n",
				finding.Confidence,
			)

			fmt.Fprintf(
				&b,
				"Category: %s\n",
				finding.Category,
			)

			fmt.Fprintf(
				&b,
				"Title: %s\n",
				finding.Title,
			)

			fmt.Fprintf(
				&b,
				"File: %s\n",
				finding.FilePath,
			)

			if finding.StartLine > 0 {
				fmt.Fprintf(
					&b,
					"Lines: %d-%d\n",
					finding.StartLine,
					finding.EndLine,
				)
			}

			fmt.Fprintf(
				&b,
				"Explanation: %s\n",
				finding.Explanation,
			)

			if finding.Suggestion != "" {
				fmt.Fprintf(
					&b,
					"Suggestion: %s\n",
					finding.Suggestion,
				)
			}

			if finding.Evidence != "" {
				fmt.Fprintf(
					&b,
					"Evidence:\n%s\n",
					finding.Evidence,
				)
			}
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
- For security-sensitive code, trace data flow from untrusted or externally influenced inputs to sensitive sinks before deciding whether the code is safe.
- When a value is used to establish a security boundary, such as a filesystem root or allowed directory, verify whether that value is itself attacker-controlled.
- Do not treat path-scoping helpers, sanitizers, validation helpers, or wrappers as proof of safety without checking what input reaches them.
- For filesystem operations, consider path traversal, absolute paths, dot segments, symlink behavior, and attacker-controlled roots where applicable.
- For subprocesses, trace arguments to the executable and distinguish fixed executables from attacker-controlled executable paths or shell interpretation.
- file_path must refer to a supplied file.
- Line numbers must refer to the supplied source.
- Prefer changed code.
- Do not invent files or line numbers.
- Do not report formatting-only issues.
- Treat deterministic analyzer findings as evidence, not automatically valid conclusions.
- Verify deterministic analyzer findings against the supplied source code before reporting them.
- Do not duplicate a deterministic finding unless additional reasoning or context makes it useful.
- Do not report the same issue multiple times.
- Do not include markdown fences.
- Return only JSON.
`)

	return b.String()
}
