package validator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Validator interface {
	Validate(
		ctx context.Context,
		request review.ReviewRequest,
		findings []review.ReviewFinding,
	) ([]review.ReviewFinding, error)

	ValidateWithContext(
		ctx context.Context,
		request review.ReviewRequest,
		reviewContext review.ReviewContext,
		findings []review.ReviewFinding,
	) ([]review.ReviewFinding, error)
}

type FindingValidator struct{}

func NewFindingValidator() *FindingValidator {
	return &FindingValidator{}
}

func (v *FindingValidator) Validate(
	ctx context.Context,
	request review.ReviewRequest,
	findings []review.ReviewFinding,
) ([]review.ReviewFinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(findings) == 0 {
		return nil, nil
	}

	validated := make([]review.ReviewFinding, 0, len(findings))

	for _, finding := range findings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := validateFinding(finding); err != nil {
			continue
		}

		validated = append(validated, finding)
	}

	return deduplicateFindings(validated), nil
}

func (v *FindingValidator) ValidateWithContext(
	ctx context.Context,
	request review.ReviewRequest,
	reviewContext review.ReviewContext,
	findings []review.ReviewFinding,
) ([]review.ReviewFinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(findings) == 0 {
		return nil, nil
	}

	validated := make([]review.ReviewFinding, 0, len(findings))

	for _, finding := range findings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := validateFinding(finding); err != nil {
			continue
		}

		if err := validateFindingContext(finding, reviewContext); err != nil {
			continue
		}

		validated = append(validated, finding)
	}

	return deduplicateFindings(validated), nil
}

func validateFinding(finding review.ReviewFinding) error {
	if strings.TrimSpace(finding.FindingID) == "" {
		return fmt.Errorf("finding ID is required")
	}

	if strings.TrimSpace(finding.Source) == "" {
		return fmt.Errorf("finding source is required")
	}

	if !isValidCategory(finding.Category) {
		return fmt.Errorf("unsupported finding category %q", finding.Category)
	}

	if !isValidSeverity(finding.Severity) {
		return fmt.Errorf("unsupported finding severity %q", finding.Severity)
	}

	if finding.Confidence < 0 || finding.Confidence > 1 {
		return fmt.Errorf(
			"confidence must be between 0 and 1, got %f",
			finding.Confidence,
		)
	}

	if strings.TrimSpace(finding.Title) == "" {
		return fmt.Errorf("finding title is required")
	}

	if strings.TrimSpace(finding.Explanation) == "" {
		return fmt.Errorf("finding explanation is required")
	}

	if strings.TrimSpace(finding.FilePath) == "" {
		return fmt.Errorf("finding file path is required")
	}

	if !isSafeRelativePath(finding.FilePath) {
		return fmt.Errorf(
			"finding file path must be a safe relative path: %q",
			finding.FilePath,
		)
	}

	if finding.StartLine < 0 {
		return fmt.Errorf("start line cannot be negative")
	}

	if finding.EndLine < 0 {
		return fmt.Errorf("end line cannot be negative")
	}

	if finding.StartLine > 0 &&
		finding.EndLine > 0 &&
		finding.EndLine < finding.StartLine {
		return fmt.Errorf(
			"end line %d is before start line %d",
			finding.EndLine,
			finding.StartLine,
		)
	}

	return nil
}

func validateFindingContext(
	finding review.ReviewFinding,
	reviewContext review.ReviewContext,
) error {
	filePath := filepath.Clean(strings.TrimSpace(finding.FilePath))

	if filePath == "." || filePath == "" {
		return fmt.Errorf("finding has an invalid file path")
	}

	changedFile, changed := findChangedFile(
		reviewContext.ChangedFiles,
		filePath,
	)

	if changed {
		// File-level findings are valid without a line range.
		if finding.StartLine == 0 && finding.EndLine == 0 {
			return nil
		}

		// When patch information is available, it is the authoritative
		// source for validating the location of a PR finding.
		//
		// Do this before checking Additions/Deletions because callers
		// may provide a patch without populating those metadata fields.
		if changedFile.Patch != "" {
			return validatePatchLineRange(
				finding,
				changedFile.Patch,
			)
		}

		// Without patch information, use the complete source file for
		// basic line-range validation when it is available.
		sourceFile, sourceFound := findSourceFile(
			reviewContext.SourceFiles,
			filePath,
		)

		if sourceFound {
			return validateLineRange(
				finding,
				sourceFile.Content,
			)
		}

		// A changed file without patch or source content cannot provide
		// enough information to validate a line-level finding.
		if changedFile.Additions == 0 &&
			changedFile.Deletions == 0 {
			return fmt.Errorf(
				"finding references a changed file without line information",
			)
		}

		return nil
	}

	// If the file is not part of the changed files, validate it against
	// the supplied source file.
	sourceFile, found := findSourceFile(
		reviewContext.SourceFiles,
		filePath,
	)

	if found {
		return validateLineRange(
			finding,
			sourceFile.Content,
		)
	}

	return fmt.Errorf(
		"finding references file %q which is not available in review context",
		finding.FilePath,
	)
}

func findSourceFile(
	files []review.SourceFile,
	path string,
) (review.SourceFile, bool) {
	for _, file := range files {
		if filepath.Clean(file.Path) == path {
			return file, true
		}
	}

	return review.SourceFile{}, false
}

func findChangedFile(
	files []review.ChangedFile,
	path string,
) (review.ChangedFile, bool) {
	for _, file := range files {
		if filepath.Clean(file.Path) == path {
			return file, true
		}
	}

	return review.ChangedFile{}, false
}

func validateLineRange(
	finding review.ReviewFinding,
	content string,
) error {
	lineCount := countLines(content)

	if finding.StartLine == 0 && finding.EndLine == 0 {
		return nil
	}

	if finding.StartLine <= 0 {
		return fmt.Errorf("start line must be greater than zero")
	}

	if finding.EndLine <= 0 {
		return fmt.Errorf("end line must be greater than zero")
	}

	if finding.StartLine > lineCount {
		return fmt.Errorf(
			"start line %d exceeds file line count %d",
			finding.StartLine,
			lineCount,
		)
	}

	if finding.EndLine > lineCount {
		return fmt.Errorf(
			"end line %d exceeds file line count %d",
			finding.EndLine,
			lineCount,
		)
	}

	return nil
}

func validatePatchLineRange(
	finding review.ReviewFinding,
	patch string,
) error {
	if finding.StartLine == 0 && finding.EndLine == 0 {
		return nil
	}

	if finding.StartLine <= 0 {
		return fmt.Errorf("start line must be greater than zero")
	}

	if finding.EndLine <= 0 {
		return fmt.Errorf("end line must be greater than zero")
	}

	if finding.EndLine < finding.StartLine {
		return fmt.Errorf(
			"end line %d is before start line %d",
			finding.EndLine,
			finding.StartLine,
		)
	}

	patchLines := countPatchNewFileLines(patch)

	if len(patchLines) == 0 {
		return fmt.Errorf("supplied patch contains no new-file lines")
	}

	for line := finding.StartLine; line <= finding.EndLine; line++ {
		if _, exists := patchLines[line]; exists {
			continue
		}

		return fmt.Errorf(
			"finding line %d is not present in the supplied patch",
			line,
		)
	}

	return nil
}

func countPatchNewFileLines(patch string) map[int]struct{} {
	lines := make(map[int]struct{})

	if strings.TrimSpace(patch) == "" {
		return lines
	}

	currentLine := 0

	for _, rawLine := range strings.Split(patch, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")

		if strings.HasPrefix(line, "@@") {
			currentLine = parsePatchNewLineStart(line)
			continue
		}

		if currentLine <= 0 {
			continue
		}

		// Added lines belong to the new file.
		if strings.HasPrefix(line, "+") &&
			!strings.HasPrefix(line, "+++") {
			lines[currentLine] = struct{}{}
			currentLine++
			continue
		}

		// Deleted lines do not advance the new-file line number.
		if strings.HasPrefix(line, "-") &&
			!strings.HasPrefix(line, "---") {
			continue
		}

		// Context lines are part of the supplied hunk and therefore
		// represent valid locations inside that hunk.
		if strings.HasPrefix(line, " ") {
			lines[currentLine] = struct{}{}
			currentLine++
		}
	}

	return lines
}

func parsePatchNewLineStart(header string) int {
	parts := strings.Fields(header)

	for _, part := range parts {
		if !strings.HasPrefix(part, "+") {
			continue
		}

		value := strings.TrimPrefix(part, "+")

		if strings.HasPrefix(value, "+") {
			value = strings.TrimPrefix(value, "+")
		}

		if comma := strings.IndexByte(value, ','); comma >= 0 {
			value = value[:comma]
		}

		var lineNumber int

		if _, err := fmt.Sscanf(value, "%d", &lineNumber); err == nil {
			return lineNumber
		}
	}

	return 0
}

func countLines(content string) int {
	if content == "" {
		return 0
	}

	return len(strings.Split(content, "\n"))
}

func isSafeRelativePath(path string) bool {
	path = strings.TrimSpace(path)

	if path == "" {
		return false
	}

	if filepath.IsAbs(path) {
		return false
	}

	cleaned := filepath.Clean(path)

	if cleaned == "." {
		return false
	}

	if cleaned == ".." ||
		strings.HasPrefix(
			cleaned,
			".."+string(filepath.Separator),
		) {
		return false
	}

	return true
}

func isValidCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "correctness",
		"security",
		"concurrency",
		"reliability",
		"error_handling",
		"maintainability",
		"test_quality",
		"performance",
		"style",
		"dependency",
		"architecture":
		return true
	default:
		return false
	}
}

func isValidSeverity(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical",
		"high",
		"medium",
		"low",
		"info":
		return true
	default:
		return false
	}
}

func deduplicateFindings(
	findings []review.ReviewFinding,
) []review.ReviewFinding {
	if len(findings) <= 1 {
		return findings
	}

	type findingKey struct {
		filePath string
		category string
		title    string
		start    int
		end      int
	}

	unique := make(map[findingKey]review.ReviewFinding, len(findings))

	for _, finding := range findings {
		key := findingKey{
			filePath: filepath.Clean(
				strings.TrimSpace(finding.FilePath),
			),
			category: strings.ToLower(
				strings.TrimSpace(finding.Category),
			),
			title: normalizeText(finding.Title),
			start: finding.StartLine,
			end:   finding.EndLine,
		}

		existing, exists := unique[key]

		if !exists {
			unique[key] = finding
			continue
		}

		if shouldReplace(existing, finding) {
			unique[key] = finding
		}
	}

	result := make([]review.ReviewFinding, 0, len(unique))

	for _, finding := range unique {
		result = append(result, finding)
	}

	return result
}

func shouldReplace(
	existing review.ReviewFinding,
	candidate review.ReviewFinding,
) bool {
	if candidate.Confidence > existing.Confidence {
		return true
	}

	if candidate.Confidence < existing.Confidence {
		return false
	}

	existingSeverity := severityRank(existing.Severity)
	candidateSeverity := severityRank(candidate.Severity)

	return candidateSeverity > existingSeverity
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func normalizeText(value string) string {
	return strings.ToLower(
		strings.Join(
			strings.Fields(strings.TrimSpace(value)),
			" ",
		),
	)
}
