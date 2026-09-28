package analyzer

import (
	"context"
	"testing"
)

func TestOSCommandRunner_Run_Success(t *testing.T) {
	runner := NewOSCommandRunner()

	result := runner.Run(
		context.Background(),
		"sh",
		"-c",
		"printf 'hello'",
	)

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Stdout != "hello" {
		t.Fatalf("expected stdout %q, got %q", "hello", result.Stdout)
	}

	if result.Stderr != "" {
		t.Fatalf("expected empty stderr, got %q", result.Stderr)
	}
}

func TestOSCommandRunner_Run_Failure(t *testing.T) {
	runner := NewOSCommandRunner()

	result := runner.Run(
		context.Background(),
		"sh",
		"-c",
		"printf 'failure' >&2; exit 2",
	)

	if result.ExitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", result.ExitCode)
	}

	if result.Stderr != "failure" {
		t.Fatalf("expected stderr %q, got %q", "failure", result.Stderr)
	}
}

func TestOSCommandRunner_Run_CommandNotFound(t *testing.T) {
	runner := NewOSCommandRunner()

	result := runner.Run(
		context.Background(),
		"command-that-does-not-exist",
	)

	if result.ExitCode != -1 {
		t.Fatalf("expected exit code -1, got %d", result.ExitCode)
	}

	if result.Stderr == "" {
		t.Fatal("expected stderr to contain command error")
	}
}
