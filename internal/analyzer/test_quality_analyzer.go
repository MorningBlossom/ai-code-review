package analyzer

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type TestQualityAnalyzer struct{}

func NewTestQualityAnalyzer() *TestQualityAnalyzer {
	return &TestQualityAnalyzer{}
}

func (a *TestQualityAnalyzer) Name() string {
	return "test-quality"
}

func (a *TestQualityAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	start := time.Now()

	result := review.AnalyzerResult{
		AnalyzerName:    a.Name(),
		AnalyzerVersion: "1.0",
		Status:          "passed",
	}

	if err := ctx.Err(); err != nil {
		result.Status = "failed"
		result.Diagnostics = append(result.Diagnostics, err.Error())
		result.DurationMillis = time.Since(start).Milliseconds()
		return result
	}

	for _, sourceFile := range reviewContext.SourceFiles {
		if !strings.HasSuffix(sourceFile.Path, "_test.go") {
			continue
		}

		findings, err := analyzeTestSource(
			sourceFile.Path,
			sourceFile.Content,
		)
		if err != nil {
			result.Diagnostics = append(
				result.Diagnostics,
				sourceFile.Path+": "+err.Error(),
			)
			continue
		}

		result.Findings = append(result.Findings, findings...)
	}

	result.DurationMillis = time.Since(start).Milliseconds()

	return result
}

func (a *TestQualityAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	start := time.Now()

	result := review.AnalyzerResult{
		AnalyzerName:    a.Name(),
		AnalyzerVersion: "1.0",
		Status:          "passed",
	}

	if err := ctx.Err(); err != nil {
		result.Status = "failed"
		result.Diagnostics = append(result.Diagnostics, err.Error())
		result.DurationMillis = time.Since(start).Milliseconds()
		return result
	}

	if ws == nil {
		result.Status = "failed"
		result.Diagnostics = append(result.Diagnostics, "workspace is nil")
	}

	result.DurationMillis = time.Since(start).Milliseconds()

	return result
}

func analyzeTestSource(
	filePath string,
	source string,
) ([]review.ReviewFinding, error) {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(
		fileSet,
		filePath,
		source,
		parser.ParseComments,
	)
	if err != nil {
		return nil, err
	}

	var findings []review.ReviewFinding

	ast.Inspect(file, func(node ast.Node) bool {
		funcDecl, ok := node.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			return true
		}

		if !isTestFunction(funcDecl) {
			return true
		}

		findings = append(
			findings,
			findTestFunctionIssues(fileSet, filePath, funcDecl)...,
		)

		return true
	})

	return findings, nil
}

func isTestFunction(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Recv != nil {
		return false
	}

	if !strings.HasPrefix(fn.Name.Name, "Test") {
		return false
	}

	if fn.Type == nil || fn.Type.Params == nil {
		return false
	}

	if fn.Type.Params.NumFields() != 1 {
		return false
	}

	param := fn.Type.Params.List[0]

	selector, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	_, ok = selector.X.(*ast.SelectorExpr)

	return ok
}

func findTestFunctionIssues(
	fileSet *token.FileSet,
	filePath string,
	fn *ast.FuncDecl,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	hasAssertion := false

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.CallExpr:
			selector, ok := current.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			identifier, ok := selector.X.(*ast.Ident)
			if !ok || identifier.Name != "t" {
				return true
			}

			switch selector.Sel.Name {
			case "Error", "Errorf", "Fail", "Failf", "Fatal", "Fatalf":
				hasAssertion = true
			}

		}

		return true
	})

	startLine := fileSet.Position(fn.Pos()).Line
	endLine := fileSet.Position(fn.End()).Line

	if !hasAssertion {
		findings = append(
			findings,
			newTestQualityFinding(
				filePath,
				startLine,
				endLine,
				"TQ001",
				"test_quality",
				"medium",
				0.93,
				"Test does not contain a test assertion or failure check",
				"This test function does not appear to call testing.T assertion or failure methods. It may execute code without verifying the expected result.",
				"Add an explicit assertion or failure condition that verifies the behavior being tested.",
			),
		)
	}

	return findings
}

func newTestQualityFinding(
	filePath string,
	startLine int,
	endLine int,
	findingID string,
	category string,
	severity string,
	confidence float64,
	title string,
	explanation string,
	suggestion string,
) review.ReviewFinding {
	return review.ReviewFinding{
		FindingID:   findingID,
		Source:      "test-quality",
		Category:    category,
		Severity:    severity,
		Confidence:  confidence,
		Title:       title,
		Explanation: explanation,
		Suggestion:  suggestion,
		FilePath:    filePath,
		StartLine:   startLine,
		EndLine:     endLine,
	}
}
