package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "test-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)

	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	err := VerifyWebhookSignature(
		payload,
		signature,
		secret,
	)

	if err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestVerifyWebhookSignature_Invalid(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "test-secret"

	err := VerifyWebhookSignature(
		payload,
		"sha256=invalid",
		secret,
	)

	if err == nil {
		t.Fatal("expected invalid signature error, got nil")
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "test-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)

	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	err := VerifyWebhookSignature(
		payload,
		signature,
		"wrong-secret",
	)

	if err == nil {
		t.Fatal("expected invalid signature error, got nil")
	}
}

func TestVerifyWebhookSignature_WrongPayload(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "test-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)

	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	err := VerifyWebhookSignature(
		[]byte(`{"action":"closed"}`),
		signature,
		secret,
	)

	if err == nil {
		t.Fatal("expected invalid signature error, got nil")
	}
}

func TestVerifyWebhookSignature_MissingPrefix(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "test-secret"

	err := VerifyWebhookSignature(
		payload,
		"invalid-prefix",
		secret,
	)

	if err == nil {
		t.Fatal("expected invalid signature error, got nil")
	}
}

func TestVerifyWebhookSignature_EmptySecret(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)

	err := VerifyWebhookSignature(
		payload,
		"sha256=anything",
		"",
	)

	if err == nil {
		t.Fatal("expected invalid signature error, got nil")
	}
}
