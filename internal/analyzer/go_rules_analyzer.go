package analyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type GoRulesAnalyzer struct {
}

func NewGoRulesAnalyzer() *GoRulesAnalyzer {
	return &GoRulesAnalyzer{}
}

func (a *GoRulesAnalyzer) Name() string {
	return "go-rules"
}

func (a *GoRulesAnalyzer) Analyze(
	ctx context.Context,
	reviewContext review.ReviewContext,
) review.AnalyzerResult {
	start := time.Now()

	result := review.AnalyzerResult{
		AnalyzerName:    a.Name(),
		AnalyzerVersion: "1.0.0",
		Status:          "completed",
	}

	for _, sourceFile := range reviewContext.SourceFiles {
		if err := ctx.Err(); err != nil {
			result.Status = "cancelled"
			result.Diagnostics = append(
				result.Diagnostics,
				err.Error(),
			)
			result.DurationMillis = time.Since(start).Milliseconds()
			return result
		}

		if !isGoFile(sourceFile.Path) {
			continue
		}

		findings, diagnostics := analyzeGoSource(
			sourceFile.Path,
			sourceFile.Content,
		)

		result.Findings = append(
			result.Findings,
			findings...,
		)

		result.Diagnostics = append(
			result.Diagnostics,
			diagnostics...,
		)
	}

	result.DurationMillis = time.Since(start).Milliseconds()

	return result
}

func (a *GoRulesAnalyzer) AnalyzeWorkspace(
	ctx context.Context,
	ws workspace.Workspace,
) review.AnalyzerResult {
	start := time.Now()
	result := review.AnalyzerResult{
		AnalyzerName:    a.Name(),
		AnalyzerVersion: "1.0.0",
		Status:          "completed",
	}

	rootPath := ws.Root()

	root, err := os.OpenRoot(rootPath)
	if err != nil {
		result.Status = "failed"
		result.Diagnostics = append(
			result.Diagnostics,
			fmt.Sprintf("failed to open workspace root %s: %v", rootPath, err),
		)
		result.DurationMillis = time.Since(start).Milliseconds()
		return result
	}
	defer func() {
		_ = root.Close()
	}()

	err = filepath.WalkDir(
		rootPath,
		func(path string, entry os.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			if walkErr != nil {
				result.Diagnostics = append(
					result.Diagnostics,
					fmt.Sprintf("failed to inspect %s: %v", path, walkErr),
				)
				return nil
			}

			if entry.IsDir() {
				if shouldSkipDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			if !isGoFile(path) {
				return nil
			}

			relativePath, err := filepath.Rel(rootPath, path)
			if err != nil {
				result.Diagnostics = append(
					result.Diagnostics,
					fmt.Sprintf("failed to determine relative path for %s: %v", path, err),
				)
				return nil
			}

			file, err := root.Open(relativePath)
			if err != nil {
				result.Diagnostics = append(
					result.Diagnostics,
					fmt.Sprintf("failed to read %s: %v", relativePath, err),
				)
				return nil
			}

			content, err := io.ReadAll(file)
			closeErr := file.Close()

			if err != nil {
				result.Diagnostics = append(
					result.Diagnostics,
					fmt.Sprintf("failed to read %s: %v", relativePath, err),
				)
				return nil
			}

			if closeErr != nil {
				result.Diagnostics = append(
					result.Diagnostics,
					fmt.Sprintf("failed to close %s: %v", relativePath, closeErr),
				)
				return nil
			}

			findings, diagnostics := analyzeGoSource(
				relativePath,
				string(content),
			)

			result.Findings = append(result.Findings, findings...)
			result.Diagnostics = append(result.Diagnostics, diagnostics...)

			return nil
		},
	)

	if err != nil {
		if ctx.Err() != nil {
			result.Status = "cancelled"
			result.Diagnostics = append(
				result.Diagnostics,
				ctx.Err().Error(),
			)
		} else {
			result.Status = "failed"
			result.Diagnostics = append(
				result.Diagnostics,
				err.Error(),
			)
		}
	}

	result.DurationMillis = time.Since(start).Milliseconds()
	return result
}

func analyzeGoSource(
	filePath string,
	content string,
) ([]review.ReviewFinding, []string) {
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(
		fset,
		filePath,
		content,
		parser.ParseComments,
	)

	if err != nil {
		return nil, []string{
			fmt.Sprintf(
				"failed to parse %s: %v",
				filePath,
				err,
			),
		}
	}

	var findings []review.ReviewFinding

	findings = append(
		findings,
		findIgnoredErrorFindings(
			fset,
			file,
			filePath,
		)...,
	)

	findings = append(
		findings,
		findHTTPClientTimeoutFindings(
			fset,
			file,
			filePath,
		)...,
	)

	findings = append(
		findings,
		findHTTPGetFindings(
			fset,
			file,
			filePath,
		)...,
	)

	findings = append(
		findings,
		findDatabaseContextFindings(
			fset,
			file,
			filePath,
		)...,
	)

	findings = append(
		findings,
		findHardcodedLocalURLFindings(
			fset,
			file,
			filePath,
		)...,
	)

	findings = append(
		findings,
		findSleepFindings(
			fset,
			file,
			filePath,
		)...,
	)

	findings = append(
		findings,
		findUnboundedGoroutineFindings(
			fset,
			file,
			filePath,
		)...,
	)

	return findings, nil
}

func findIgnoredErrorFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}

			for index, expr := range assign.Rhs {
				call, ok := expr.(*ast.CallExpr)
				if !ok {
					continue
				}

				if index >= len(assign.Lhs) {
					continue
				}

				identifier, ok := assign.Lhs[index].(*ast.Ident)
				if !ok || identifier.Name != "_" {
					continue
				}

				start, end := nodeLines(
					fset,
					assign,
				)

				findings = append(
					findings,
					newFinding(
						filePath,
						start,
						end,
						"GR001",
						"correctness",
						"medium",
						0.94,
						"Ignored return value may hide an error",
						"The return value of a function call is assigned to `_`. If the call can fail, ignoring the result may hide an error condition.",
						"Handle the returned error explicitly or document why the return value is intentionally ignored.",
					),
				)

				_ = call
			}

			return true
		},
	)

	return findings
}

func findHTTPClientTimeoutFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			composite, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}

			selector, ok := composite.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			packageIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			if packageIdent.Name != "http" ||
				selector.Sel.Name != "Client" {
				return true
			}

			hasTimeout := false

			for _, element := range composite.Elts {
				kv, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}

				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}

				if key.Name == "Timeout" {
					hasTimeout = true
					break
				}
			}

			if hasTimeout {
				return true
			}

			start, end := nodeLines(
				fset,
				composite,
			)

			findings = append(
				findings,
				newFinding(
					filePath,
					start,
					end,
					"GR002",
					"reliability",
					"medium",
					0.91,
					"HTTP client has no explicit timeout",
					"An http.Client without an explicit timeout can allow requests to remain blocked indefinitely.",
					"Configure an appropriate Timeout or use a context with a deadline for the request.",
				),
			)

			return true
		},
	)

	return findings
}

func findHTTPGetFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			packageIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			if packageIdent.Name != "http" ||
				selector.Sel.Name != "Get" {
				return true
			}

			start, end := nodeLines(
				fset,
				call,
			)

			findings = append(
				findings,
				newFinding(
					filePath,
					start,
					end,
					"GR003",
					"reliability",
					"medium",
					0.88,
					"HTTP request does not expose request context",
					"http.Get creates a request without allowing the caller to propagate cancellation or deadlines through context.",
					"Prefer http.NewRequestWithContext and an HTTP client with an explicit timeout.",
				),
			)

			return true
		},
	)

	return findings
}

func findDatabaseContextFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			switch selector.Sel.Name {
			case "Query", "QueryRow", "Exec":
			default:
				return true
			}

			if len(call.Args) == 0 {
				return true
			}

			if !looksLikeDatabaseReceiver(selector.X) {
				return true
			}

			start, end := nodeLines(
				fset,
				call,
			)

			findings = append(
				findings,
				newFinding(
					filePath,
					start,
					end,
					"GR004",
					"reliability",
					"medium",
					0.87,
					"Database operation does not propagate context",
					"Database operations without a context cannot directly inherit request cancellation or deadlines.",
					"Prefer QueryContext, QueryRowContext, or ExecContext when the database API supports them.",
				),
			)

			return true
		},
	)

	return findings
}

func findHardcodedLocalURLFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}

			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}

			if !isHardcodedLocalURL(value) {
				return true
			}

			start, end := nodeLines(
				fset,
				literal,
			)

			findings = append(
				findings,
				newFinding(
					filePath,
					start,
					end,
					"GR005",
					"architecture",
					"low",
					0.86,
					"Hard-coded local service URL",
					"A localhost or loopback service URL is embedded directly in application code. This can make deployment configuration environment-specific.",
					"Move the service endpoint into configuration or environment variables.",
				),
			)

			return true
		},
	)

	return findings
}

func findSleepFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			packageIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			if packageIdent.Name != "time" ||
				selector.Sel.Name != "Sleep" {
				return true
			}

			start, end := nodeLines(
				fset,
				call,
			)

			findings = append(
				findings,
				newFinding(
					filePath,
					start,
					end,
					"GR006",
					"reliability",
					"low",
					0.82,
					"time.Sleep may block service execution",
					"Direct sleeps in service logic can block execution and may not respond to request cancellation.",
					"Consider context-aware waiting, timers, or a bounded retry/backoff mechanism where appropriate.",
				),
			)

			return true
		},
	)

	return findings
}

func findUnboundedGoroutineFindings(
	fset *token.FileSet,
	file *ast.File,
	filePath string,
) []review.ReviewFinding {
	var findings []review.ReviewFinding

	ast.Inspect(
		file,
		func(node ast.Node) bool {
			goStmt, ok := node.(*ast.GoStmt)
			if !ok {
				return true
			}

			if !isInsideLoop(file, goStmt) {
				return true
			}

			start, end := nodeLines(
				fset,
				goStmt,
			)

			findings = append(
				findings,
				newFinding(
					filePath,
					start,
					end,
					"GR007",
					"concurrency",
					"medium",
					0.79,
					"Goroutine started inside a loop",
					"Starting a goroutine for every loop iteration can create an unbounded number of concurrent tasks when the input is large or long-lived.",
					"Consider a bounded worker pool, semaphore, errgroup limit, or another explicit concurrency control mechanism.",
				),
			)

			return true
		},
	)

	return findings
}

func looksLikeDatabaseReceiver(expr ast.Expr) bool {
	identifier, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}

	name := strings.ToLower(identifier.Name)

	return strings.Contains(name, "db") ||
		strings.Contains(name, "database") ||
		strings.Contains(name, "sql")
}

func isHardcodedLocalURL(value string) bool {
	value = strings.TrimSpace(value)

	if value == "" {
		return false
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}

	if parsed.Scheme != "http" &&
		parsed.Scheme != "https" {
		return false
	}

	host := strings.ToLower(parsed.Hostname())

	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func isInsideLoop(
	file *ast.File,
	target ast.Node,
) bool {
	found := false

	var inspect func(ast.Node, bool)

	inspect = func(node ast.Node, insideLoop bool) {
		if node == nil || found {
			return
		}

		if node == target {
			found = insideLoop
			return
		}

		nextInsideLoop := insideLoop

		switch node.(type) {
		case *ast.ForStmt,
			*ast.RangeStmt:
			nextInsideLoop = true
		}

		ast.Inspect(
			node,
			func(child ast.Node) bool {
				if child == nil || found {
					return false
				}

				if child == node {
					return true
				}

				inspect(child, nextInsideLoop)
				return false
			},
		)
	}

	inspect(file, false)

	return found
}

func nodeLines(
	fset *token.FileSet,
	node ast.Node,
) (int, int) {
	if node == nil {
		return 0, 0
	}

	start := fset.Position(node.Pos()).Line
	end := fset.Position(node.End()).Line

	if start < 1 {
		start = 1
	}

	if end < start {
		end = start
	}

	return start, end
}

func newFinding(
	filePath string,
	startLine int,
	endLine int,
	id string,
	category string,
	severity string,
	confidence float64,
	title string,
	explanation string,
	suggestion string,
) review.ReviewFinding {
	return review.ReviewFinding{
		FindingID:   id,
		Source:      "go-rules",
		Category:    category,
		Severity:    severity,
		Confidence:  confidence,
		Title:       title,
		Explanation: explanation,
		Suggestion:  suggestion,
		FilePath:    filepath.ToSlash(filePath),
		StartLine:   startLine,
		EndLine:     endLine,
	}
}

func isGoFile(path string) bool {
	return strings.EqualFold(
		filepath.Ext(path),
		".go",
	)
}

func shouldSkipDirectory(name string) bool {
	switch name {
	case ".git",
		".hg",
		".svn",
		"vendor",
		"node_modules",
		"dist",
		"build",
		"tmp",
		"coverage":
		return true
	default:
		return false
	}
}
