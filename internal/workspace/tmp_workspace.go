package workspace

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

type TempWorkspace struct {
	root string
}

func NewTempWorkspace() (*TempWorkspace, error) {
	root, err := os.MkdirTemp("", "ai-code-review-*")
	if err != nil {
		return nil, err
	}

	return &TempWorkspace{
		root: root,
	}, nil
}

func (w *TempWorkspace) Root() string {
	return w.root
}

func (w *TempWorkspace) Run(
	ctx context.Context,
	name string,
	args ...string,
) CommandResult {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = w.root

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err == nil {
		result.ExitCode = 0
		return result
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitError.ExitCode()
		return result
	}

	result.ExitCode = -1
	result.Stderr = err.Error()

	return result
}

func (w *TempWorkspace) Close() error {
	return os.RemoveAll(w.root)
}
