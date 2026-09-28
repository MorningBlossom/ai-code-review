package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTempWorkspace_CreatesAndCleansDirectory(t *testing.T) {
	ws, err := NewTempWorkspace()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	root := ws.Root()

	if root == "" {
		t.Fatal("expected workspace root")
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("expected workspace directory to exist: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("expected workspace root to be a directory")
	}

	if err := ws.Close(); err != nil {
		t.Fatalf("expected close to succeed, got %v", err)
	}

	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("expected workspace directory to be removed")
	}
}

func TestTempWorkspace_Run(t *testing.T) {
	ws, err := NewTempWorkspace()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer ws.Close()

	filePath := filepath.Join(ws.Root(), "test.txt")

	if err := os.WriteFile(
		filePath,
		[]byte("hello"),
		0644,
	); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result := ws.Run(
		context.Background(),
		"cat",
		"test.txt",
	)

	if result.ExitCode != 0 {
		t.Fatalf(
			"expected exit code 0, got %d; stderr=%q",
			result.ExitCode,
			result.Stderr,
		)
	}

	if result.Stdout != "hello" {
		t.Fatalf(
			"expected stdout %q, got %q",
			"hello",
			result.Stdout,
		)
	}
}

func TestTempWorkspace_RunFailure(t *testing.T) {
	ws, err := NewTempWorkspace()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer ws.Close()

	result := ws.Run(
		context.Background(),
		"sh",
		"-c",
		"printf 'failure' >&2; exit 2",
	)

	if result.ExitCode != 2 {
		t.Fatalf(
			"expected exit code 2, got %d",
			result.ExitCode,
		)
	}

	if result.Stderr != "failure" {
		t.Fatalf(
			"expected stderr %q, got %q",
			"failure",
			result.Stderr,
		)
	}
}
