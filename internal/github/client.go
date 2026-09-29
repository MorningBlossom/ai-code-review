package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/MorningBlossom/ai-code-review/internal/review"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	httpClient    HTTPClient
	authenticator AppAuthenticator
	baseURL       string
}

func NewClient(
	httpClient HTTPClient,
	authenticator AppAuthenticator,
	baseURL string,
) *Client {
	return &Client{
		httpClient:    httpClient,
		authenticator: authenticator,
		baseURL:       baseURL,
	}
}

type repositoryContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type pullRequestFilesResponse struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`

	Additions int `json:"additions"`
	Deletions int `json:"deletions"`

	Patch string `json:"patch"`
}

type pullRequestResponse struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	Base struct {
		SHA string `json:"sha"`
	} `json:"base"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func (c *Client) GetPullRequest(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) (PullRequest, error) {
	token, err := c.authenticator.GetInstallationToken(
		ctx,
		installationID,
	)
	if err != nil {
		return PullRequest{}, fmt.Errorf(
			"get installation token: %w",
			err,
		)
	}

	url := fmt.Sprintf(
		"%s/repos/%s/%s/pulls/%d",
		c.baseURL,
		organization,
		repository,
		pullRequestNumber,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return PullRequest{}, fmt.Errorf(
			"create GitHub request: %w",
			err,
		)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PullRequest{}, fmt.Errorf(
			"request GitHub pull request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PullRequest{}, fmt.Errorf(
			"GitHub pull request returned status %d",
			resp.StatusCode,
		)
	}

	var result pullRequestResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return PullRequest{}, fmt.Errorf(
			"decode GitHub pull request: %w",
			err,
		)
	}

	return PullRequest{
		Number:  result.Number,
		Title:   result.Title,
		Body:    result.Body,
		BaseSHA: result.Base.SHA,
		HeadSHA: result.Head.SHA,
		Author:  result.User.Login,
	}, nil
}

func (c *Client) GetChangedFiles(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	pullRequestNumber int,
) ([]review.ChangedFile, error) {
	token, err := c.authenticator.GetInstallationToken(
		ctx,
		installationID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get installation token: %w",
			err,
		)
	}

	const perPage = 100

	var result []review.ChangedFile

	for page := 1; ; page++ {
		url := fmt.Sprintf(
			"%s/repos/%s/%s/pulls/%d/files?per_page=%d&page=%d",
			c.baseURL,
			organization,
			repository,
			pullRequestNumber,
			perPage,
			page,
		)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			url,
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create GitHub changed files request: %w",
				err,
			)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf(
				"request GitHub changed files page %d: %w",
				page,
				err,
			)
		}

		if resp.StatusCode != http.StatusOK {
			closeErr := resp.Body.Close()

			if closeErr != nil {
				return nil, fmt.Errorf(
					"GitHub changed files page %d returned status %d; close response body: %w",
					page,
					resp.StatusCode,
					closeErr,
				)
			}

			return nil, fmt.Errorf(
				"GitHub changed files page %d returned status %d",
				page,
				resp.StatusCode,
			)
		}

		var files []pullRequestFilesResponse

		err = json.NewDecoder(resp.Body).Decode(&files)

		closeErr := resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf(
				"decode GitHub changed files page %d: %w",
				page,
				err,
			)
		}

		if closeErr != nil {
			return nil, fmt.Errorf(
				"close GitHub changed files page %d response body: %w",
				page,
				closeErr,
			)
		}

		for _, file := range files {
			result = append(result, review.ChangedFile{
				Path:      file.Filename,
				Status:    file.Status,
				Additions: file.Additions,
				Deletions: file.Deletions,
				Patch:     file.Patch,
			})
		}

		if len(files) < perPage {
			break
		}
	}

	return result, nil
}

func (c *Client) GetFileContent(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
	path string,
) (string, error) {
	token, err := c.authenticator.GetInstallationToken(
		ctx,
		installationID,
	)
	if err != nil {
		return "", fmt.Errorf(
			"get installation token: %w",
			err,
		)
	}

	url := fmt.Sprintf(
		"%s/repos/%s/%s/contents/%s?ref=%s",
		c.baseURL,
		organization,
		repository,
		path,
		ref,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf(
			"create GitHub file content request: %w",
			err,
		)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"request GitHub file content: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"GitHub file content returned status %d",
			resp.StatusCode,
		)
	}

	var result repositoryContentResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf(
			"decode GitHub file content: %w",
			err,
		)
	}

	if result.Encoding != "base64" {
		return "", fmt.Errorf(
			"unsupported GitHub file encoding: %s",
			result.Encoding,
		)
	}

	content := strings.ReplaceAll(result.Content, "\n", "")

	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", fmt.Errorf(
			"decode GitHub file content: %w",
			err,
		)
	}

	return string(decoded), nil
}

func (c *Client) DownloadRepositoryArchive(
	ctx context.Context,
	installationID int64,
	organization string,
	repository string,
	ref string,
) ([]byte, error) {
	token, err := c.authenticator.GetInstallationToken(
		ctx,
		installationID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get installation token: %w",
			err,
		)
	}

	archiveURL := fmt.Sprintf(
		"%s/repos/%s/%s/tarball/%s",
		c.baseURL,
		organization,
		repository,
		url.PathEscape(ref),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		archiveURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create repository archive request: %w",
			err,
		)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+token,
	)
	req.Header.Set(
		"Accept",
		"application/vnd.github+json",
	)
	req.Header.Set(
		"X-GitHub-Api-Version",
		"2022-11-28",
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"request repository archive: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"GitHub returned repository archive status %d",
			resp.StatusCode,
		)
	}

	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"read repository archive: %w",
			err,
		)
	}

	if len(bytes.TrimSpace(archive)) == 0 {
		return nil, fmt.Errorf(
			"GitHub returned an empty repository archive",
		)
	}

	return archive, nil
}
