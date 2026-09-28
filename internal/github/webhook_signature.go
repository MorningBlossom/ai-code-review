package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidWebhookSignature = errors.New("invalid webhook signature")

func VerifyWebhookSignature(
	payload []byte,
	signature string,
	secret string,
) error {
	if secret == "" {
		return ErrInvalidWebhookSignature
	}

	const prefix = "sha256="

	if !strings.HasPrefix(signature, prefix) {
		return ErrInvalidWebhookSignature
	}

	signatureBytes, err := hex.DecodeString(
		strings.TrimPrefix(signature, prefix),
	)
	if err != nil {
		return ErrInvalidWebhookSignature
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)

	expected := mac.Sum(nil)

	if !hmac.Equal(signatureBytes, expected) {
		return ErrInvalidWebhookSignature
	}

	return nil
}
