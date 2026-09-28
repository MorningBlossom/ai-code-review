package contextbuilder

import (
	"context"
	"fmt"
	"sort"

	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Builder struct {
	github    github.Provider
	maxFiles  int
	maxBytes  int
	maxTokens int
}

func NewBuilder(
	githubProvider github.Provider,
	maxFiles int,
	maxBytes int,
	maxTokens int,
) *Builder {
	return &Builder{
		github:    githubProvider,
		maxFiles:  maxFiles,
		maxBytes:  maxBytes,
		maxTokens: maxTokens,
	}
}

func (b *Builder) Build(
	ctx context.Context,
	request review.ReviewRequest,
	pr github.PullRequest,
	changedFiles []review.ChangedFile,
) (review.ReviewContext, error) {
	reviewContext := review.ReviewContext{
		Organization:      request.Organization,
		Repository:        request.Repository,
		PullRequestNumber: request.PullRequestNumber,

		PullRequestTitle: pr.Title,
		PullRequestBody:  pr.Body,

		HeadSHA: pr.HeadSHA,
		BaseSHA: pr.BaseSHA,

		ChangedFiles: changedFiles,

		ContextBudget: review.ContextBudget{
			MaxFiles:  b.maxFiles,
			MaxBytes:  b.maxBytes,
			MaxTokens: b.maxTokens,
		},
	}

	files := append([]review.ChangedFile(nil), changedFiles...)

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	for _, changedFile := range files {
		if changedFile.Status == "removed" {
			continue
		}

		if len(reviewContext.SourceFiles) >= b.maxFiles {
			break
		}

		content, err := b.github.GetFileContent(
			ctx,
			request.InstallationID,
			request.Organization,
			request.Repository,
			request.HeadSHA,
			changedFile.Path,
		)
		if err != nil {
			return review.ReviewContext{}, fmt.Errorf(
				"get source file %s: %w",
				changedFile.Path,
				err,
			)
		}

		fileBytes := len([]byte(content))

		if reviewContext.ContextBudget.UsedBytes+fileBytes > b.maxBytes {
			break
		}

		estimatedTokens := estimateTokens(content)

		if reviewContext.ContextBudget.EstimatedTokens+estimatedTokens > b.maxTokens {
			break
		}

		reviewContext.SourceFiles = append(
			reviewContext.SourceFiles,
			review.SourceFile{
				Path:    changedFile.Path,
				Content: content,
			},
		)

		reviewContext.ContextBudget.UsedBytes += fileBytes
		reviewContext.ContextBudget.EstimatedTokens += estimatedTokens
	}

	return reviewContext, nil
}

func estimateTokens(content string) int {
	if content == "" {
		return 0
	}

	// Conservative approximation for initial MVP:
	// approximately four characters per token.
	return (len(content) + 3) / 4
}
