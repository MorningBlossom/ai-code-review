package github

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Provider interface {
	GetPullRequest(
		ctx context.Context,
		organization string,
		repository string,
		pullRequestNumber int,
	) (PullRequest, error)

	GetChangedFiles(
		ctx context.Context,
		organization string,
		repository string,
		pullRequestNumber int,
	) ([]review.ChangedFile, error)

	GetFileContent(
		ctx context.Context,
		organization string,
		repository string,
		ref string,
		path string,
	) (string, error)
}

type PullRequest struct {
	Number  int
	Title   string
	BaseSHA string
	HeadSHA string
	Author  string
}
