package github

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGitHubAppAuthenticator_GenerateJWT(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	auth := &GitHubAppAuthenticator{
		appID:      123456,
		privateKey: privateKey,
	}

	tokenString, err := auth.generateJWT()
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	if tokenString == "" {
		t.Fatal("expected JWT, got empty string")
	}

	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (interface{}, error) {
			return &privateKey.PublicKey, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to parse JWT: %v", err)
	}

	if !token.Valid {
		t.Fatal("expected JWT to be valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected MapClaims")
	}

	if claims["iss"] != "123456" {
		t.Fatalf(
			"expected issuer 123456, got %v",
			claims["iss"],
		)
	}
}
