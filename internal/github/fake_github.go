package github

import (
	"context"
	"errors"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type FakeProvider struct {
	GetPullRequestError            error
	GetChangedFilesError           error
	Archive                        []byte
	DownloadRepositoryArchiveError error
}

func (f *FakeProvider) GetPullRequest(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) (PullRequest, error) {
	if f.GetPullRequestError != nil {
		return PullRequest{}, f.GetPullRequestError
	}

	return PullRequest{
		Number:  pullRequestNumber,
		Title:   "Add payment retry handling",
		Body:    "This PR adds retry handling for failed payments.",
		BaseSHA: "base-123",
		HeadSHA: "head-456",
		Author:  "test-user",
	}, nil
}

func (f *FakeProvider) GetChangedFiles(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) ([]review.ChangedFile, error) {
	if f.GetChangedFilesError != nil {
		return nil, f.GetChangedFilesError
	}

	return []review.ChangedFile{
		{
			Path:      "payment/retry.go",
			Status:    "modified",
			Additions: 20,
			Deletions: 5,
			Patch:     "+ retryPayment()",
		},
	}, nil
}

func (f *FakeProvider) GetFileContent(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
	path string,
) (string, error) {
	return `package payment

func retryPayment() error {
	return nil
}`, nil
}

func (f *FakeProvider) DownloadRepositoryArchive(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) ([]byte, error) {
	if f.DownloadRepositoryArchiveError != nil {
		return nil, f.DownloadRepositoryArchiveError
	}

	return f.Archive, nil
}

var ErrFakeGitHub = errors.New("fake github error")
