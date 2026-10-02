package github

import (
	"context"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type Provider interface {
	GetPullRequest(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		pullRequestNumber int,
	) (PullRequest, error)

	GetChangedFiles(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		pullRequestNumber int,
	) ([]review.ChangedFile, error)

	GetFileContent(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		ref string,
		path string,
	) (string, error)

	DownloadRepositoryArchive(
		ctx context.Context,
		installationID int64,
		organization string,
		repository string,
		ref string,
	) ([]byte, error)
}

type PullRequest struct {
	Number  int
	Title   string
	Body    string
	BaseSHA string
	HeadSHA string
	Author  string
	Draft   bool
}

type PullRequestReview struct {
	Body     string
	Event    string
	Comments []PullRequestReviewComment
}

type PullRequestReviewComment struct {
	Path string
	Line int
	Side string
	Body string
}

type PullRequestReviewInfo struct {
	ID        int64
	UserLogin string
	Body      string
	CommitSHA string
}
