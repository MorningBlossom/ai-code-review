package analyzer

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type SecurityRulesAnalyzer struct {
	mu sync.Mutex
}

func NewSecurityRulesAnalyzer() *SecurityRulesAnalyzer {
	return &SecurityRulesAnalyzer{}
}

func (a *SecurityRulesAnalyzer) Name() string {
	return "security-rules"
}

func (a *SecurityRulesAnalyzer) Analyze(
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

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, sourceFile := range reviewContext.SourceFiles {
		if !isGoFile(sourceFile.Path) {
			continue
		}

		findings, err := analyzeSecuritySource(
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

func (a *SecurityRulesAnalyzer) AnalyzeWorkspace(
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
		result.Diagnostics = append(
			result.Diagnostics,
			"workspace is nil",
		)
		result.DurationMillis = time.Since(start).Milliseconds()
		return result
	}

	result.DurationMillis = time.Since(start).Milliseconds()

	return result
}

func analyzeSecuritySource(
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
		switch current := node.(type) {
		case *ast.CompositeLit:
			findings = append(
				findings,
				findInsecureTLSConfig(fileSet, filePath, current)...,
			)

		case *ast.CallExpr:
			findings = append(
				findings,
				findInsecureCommandExecution(fileSet, filePath, current)...,
			)

		case *ast.SelectorExpr:
			findings = append(
				findings,
				findWeakRandomUsage(fileSet, filePath, current)...,
			)

		case *ast.BasicLit:
			findings = append(
				findings,
				findHardcodedSecret(fileSet, filePath, current)...,
			)
		}

		return true
	})

	return findings, nil
}

func findInsecureTLSConfig(
	fileSet *token.FileSet,
	filePath string,
	node *ast.CompositeLit,
) []review.ReviewFinding {
	selector, ok := node.Type.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	packageIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil
	}

	if packageIdent.Name != "tls" || selector.Sel.Name != "Config" {
		return nil
	}

	for _, element := range node.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := keyValue.Key.(*ast.Ident)
		if !ok || key.Name != "InsecureSkipVerify" {
			continue
		}

		value, ok := keyValue.Value.(*ast.Ident)
		if !ok || value.Name != "true" {
			continue
		}

		startLine, endLine := securityNodeLines(fileSet, node)

		return []review.ReviewFinding{
			newSecurityFinding(
				filePath,
				startLine,
				endLine,
				"SEC001",
				"security",
				"high",
				0.99,
				"TLS certificate verification is disabled",
				"Setting tls.Config.InsecureSkipVerify to true disables certificate and hostname verification and can expose the connection to man-in-the-middle attacks.",
				"Avoid InsecureSkipVerify in production. Configure trusted CA certificates or use proper certificate validation instead.",
			),
		}
	}

	return nil
}

func findInsecureCommandExecution(
	fileSet *token.FileSet,
	filePath string,
	node *ast.CallExpr,
) []review.ReviewFinding {
	selector, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	packageIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil
	}

	if packageIdent.Name != "exec" {
		return nil
	}

	switch selector.Sel.Name {
	case "Command", "CommandContext":
	default:
		return nil
	}

	if len(node.Args) == 0 {
		return nil
	}

	// A literal executable such as exec.Command("git", ...)
	// is not considered dynamically controlled.
	if isStaticStringExpression(node.Args[0]) {
		return nil
	}

	startLine, endLine := securityNodeLines(fileSet, node)

	return []review.ReviewFinding{
		newSecurityFinding(
			filePath,
			startLine,
			endLine,
			"SEC002",
			"security",
			"high",
			0.93,
			"Command execution uses a non-static executable path",
			"The executable passed to os/exec is not statically defined. If this value can be influenced by external input, it may allow unintended command execution.",
			"Use a fixed executable and pass untrusted values as separate arguments. Validate or allowlist any externally supplied command-related values.",
		),
	}
}

func findWeakRandomUsage(
	fileSet *token.FileSet,
	filePath string,
	node *ast.SelectorExpr,
) []review.ReviewFinding {
	packageIdent, ok := node.X.(*ast.Ident)
	if !ok {
		return nil
	}

	if packageIdent.Name != "rand" {
		return nil
	}

	switch node.Sel.Name {
	case "Int",
		"Int31",
		"Int63",
		"Intn",
		"Int31n",
		"Int63n",
		"Float32",
		"Float64",
		"Read":
	default:
		return nil
	}

	startLine, endLine := securityNodeLines(fileSet, node)

	return []review.ReviewFinding{
		newSecurityFinding(
			filePath,
			startLine,
			endLine,
			"SEC003",
			"security",
			"medium",
			0.82,
			"Non-cryptographic random generator may be used for security-sensitive data",
			"math/rand is not designed to provide cryptographically secure randomness. Using it for tokens, reset codes, authorization values, or other security-sensitive data can make those values predictable.",
			"Use crypto/rand when unpredictability is required for security-sensitive values.",
		),
	}
}

func findHardcodedSecret(
	fileSet *token.FileSet,
	filePath string,
	node *ast.BasicLit,
) []review.ReviewFinding {
	if node.Kind != token.STRING {
		return nil
	}

	value, err := unquoteString(node.Value)
	if err != nil {
		return nil
	}

	if len(value) < 8 {
		return nil
	}

	if isURL(value) {
		return nil
	}

	if looksLikePlaceholder(value) {
		return nil
	}

	lower := strings.ToLower(value)

	secretMarkers := []string{
		"api_key=",
		"apikey=",
		"api-key=",
		"access_token=",
		"access-token=",
		"client_secret=",
		"client-secret=",
		"private_key=",
		"private-key=",
		"password=",
		"passwd=",
		"secret=",
		"bearer ",
	}

	for _, marker := range secretMarkers {
		if !strings.Contains(lower, marker) {
			continue
		}

		startLine, endLine := securityNodeLines(fileSet, node)

		return []review.ReviewFinding{
			newSecurityFinding(
				filePath,
				startLine,
				endLine,
				"SEC004",
				"security",
				"high",
				0.91,
				"Possible hard-coded credential or secret",
				"This string contains a secret-like value that appears to be embedded directly in source code.",
				"Move credentials and secrets to a secret manager or environment-based configuration and rotate any credential that may already have been committed.",
			),
		}
	}

	return nil
}

func newSecurityFinding(
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
		Source:      "security-rules",
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

func securityNodeLines(
	fileSet *token.FileSet,
	node ast.Node,
) (int, int) {
	if node == nil {
		return 0, 0
	}

	start := fileSet.Position(node.Pos()).Line
	end := fileSet.Position(node.End()).Line

	if start <= 0 {
		start = 1
	}

	if end < start {
		end = start
	}

	return start, end
}

func isStaticStringExpression(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.BasicLit:
		return value.Kind == token.STRING

	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return false
		}

		return isStaticStringExpression(value.X) &&
			isStaticStringExpression(value.Y)

	default:
		return false
	}
}

func isURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}

	return parsed.Scheme == "http" ||
		parsed.Scheme == "https" ||
		parsed.Scheme == "ftp"
}

func looksLikePlaceholder(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))

	placeholders := []string{
		"changeme",
		"change-me",
		"your-secret",
		"your_secret",
		"your-api-key",
		"your_api_key",
		"example",
		"dummy",
		"test-secret",
		"test_secret",
		"placeholder",
	}

	for _, placeholder := range placeholders {
		if strings.Contains(lower, placeholder) {
			return true
		}
	}

	return false
}

func unquoteString(value string) (string, error) {
	if len(value) < 2 {
		return "", nil
	}

	if value[0] == '`' && value[len(value)-1] == '`' {
		return value[1 : len(value)-1], nil
	}

	if value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1], nil
	}

	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}

	return value, nil
}
