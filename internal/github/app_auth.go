package github

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AppAuthenticator interface {
	GetInstallationToken(
		ctx context.Context,
		installationID int64,
	) (string, error)
}

type AppConfig struct {
	AppID          int64
	PrivateKeyPath string
}

type GitHubAppAuthenticator struct {
	appID      int64
	privateKey *rsa.PrivateKey
}

func NewGitHubAppAuthenticator(
	config AppConfig,
) (*GitHubAppAuthenticator, error) {
	privateKeyBytes, err := os.ReadFile(config.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyBytes)
	if err != nil {
		return nil, err
	}

	return &GitHubAppAuthenticator{
		appID:      config.AppID,
		privateKey: privateKey,
	}, nil
}

func (a *GitHubAppAuthenticator) generateJWT() (string, error) {
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(a.appID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodRS256,
		claims,
	)

	return token.SignedString(a.privateKey)
}

type installationTokenResponse struct {
	Token string `json:"token"`
}

func (a *GitHubAppAuthenticator) GetInstallationToken(
	ctx context.Context,
	installationID int64,
) (string, error) {
	appJWT, err := a.generateJWT()
	if err != nil {
		return "", fmt.Errorf("generate GitHub App JWT: %w", err)
	}

	url := fmt.Sprintf(
		"https://api.github.com/app/installations/%d/access_tokens",
		installationID,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(nil),
	)
	if err != nil {
		return "", fmt.Errorf("create installation token request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf(
			"GitHub returned installation token status %d",
			resp.StatusCode,
		)
	}

	var result installationTokenResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf(
			"decode installation token response: %w",
			err,
		)
	}

	if result.Token == "" {
		return "", fmt.Errorf("GitHub returned an empty installation token")
	}

	return result.Token, nil
}
