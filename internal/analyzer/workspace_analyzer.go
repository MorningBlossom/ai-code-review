package analyzer

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type WorkspaceAnalyzer interface {
	Name() string

	AnalyzeWorkspace(
		ctx context.Context,
		ws workspace.Workspace,
	) review.AnalyzerResult
}
