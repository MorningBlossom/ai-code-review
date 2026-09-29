package analyzer

import (
	"context"
	"testing"
)

func TestOSCommandRunner_Run_Success(t *testing.T) {
	runner := NewOSCommandRunner()

	result := runner.Run(
		context.Background(),
		"go",
		"version",
	)

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Stdout == "" {
		t.Fatal("expected stdout to contain Go version")
	}

	if result.Stderr != "" {
		t.Fatalf("expected empty stderr, got %q", result.Stderr)
	}
}

func TestOSCommandRunner_Run_Failure(t *testing.T) {
	runner := NewOSCommandRunner()

	result := runner.Run(
		context.Background(),
		"gofmt",
		"-unknown-flag",
	)

	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}

	if result.Stderr == "" {
		t.Fatal("expected stderr to contain command error")
	}
}

func TestOSCommandRunner_Run_CommandNotAllowed(t *testing.T) {
	runner := NewOSCommandRunner()

	result := runner.Run(
		context.Background(),
		"sh",
		"-c",
		"printf 'hello'",
	)

	if result.ExitCode != -1 {
		t.Fatalf("expected exit code -1, got %d", result.ExitCode)
	}

	if result.Stderr == "" {
		t.Fatal("expected stderr to contain command validation error")
	}
}
