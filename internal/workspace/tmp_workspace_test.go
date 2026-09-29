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

	if err := os.WriteFile(
		filepath.Join(ws.Root(), "go.mod"),
		[]byte("module example.com/test\n\ngo 1.27\n"),
		0600,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(ws.Root(), "main.go"),
		[]byte(`package main

func main() {}
`),
		0600,
	); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	result := ws.Run(
		context.Background(),
		"go",
		"test",
		"./...",
	)

	if result.ExitCode != 0 {
		t.Fatalf(
			"expected exit code 0, got %d; stdout=%q; stderr=%q",
			result.ExitCode,
			result.Stdout,
			result.Stderr,
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
		"not-allowed-command",
	)

	if result.ExitCode != -1 {
		t.Fatalf(
			"expected exit code -1, got %d",
			result.ExitCode,
		)
	}

	if result.Stderr == "" {
		t.Fatal("expected stderr to contain command validation error")
	}
}
