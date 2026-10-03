package workspace

import (
	"context"
	"fmt"
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
	cmd, err := newAllowedWorkspaceCommand(ctx, name, args...)
	if err != nil {
		return CommandResult{
			ExitCode: -1,
			Stderr:   err.Error(),
		}
	}

	cmd.Dir = w.root
	cmd.Env = constrainedAnalyzerEnv()

	stdout, err := cmd.Output()

	result := CommandResult{
		Stdout: string(stdout),
	}

	if err == nil {
		result.ExitCode = 0
		return result
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitError.ExitCode()
		result.Stderr = string(exitError.Stderr)
		return result
	}

	result.ExitCode = -1
	result.Stderr = err.Error()

	return result
}

func (w *TempWorkspace) Close() error {
	return os.RemoveAll(w.root)
}

func newAllowedWorkspaceCommand(
	ctx context.Context,
	name string,
	args ...string,
) (*exec.Cmd, error) {
	switch {
	case name == "go" &&
		len(args) == 2 &&
		args[0] == "vet" &&
		args[1] == "./...":
		return exec.CommandContext(ctx, "go", "vet", "./..."), nil

	case name == "go" &&
		len(args) == 2 &&
		args[0] == "test" &&
		args[1] == "./...":
		return exec.CommandContext(ctx, "go", "test", "./..."), nil

	case name == "go" &&
		len(args) == 3 &&
		args[0] == "test" &&
		args[1] == "-race" &&
		args[2] == "./...":
		return exec.CommandContext(ctx, "go", "test", "-race", "./..."), nil

	case name == "staticcheck" &&
		len(args) == 1 &&
		args[0] == "./...":
		return exec.CommandContext(ctx, "staticcheck", "./..."), nil

	case name == "gosec" &&
		len(args) == 1 &&
		args[0] == "./...":
		return exec.CommandContext(ctx, "gosec", "./..."), nil

	case name == "govulncheck" &&
		len(args) == 1 &&
		args[0] == "./...":
		return exec.CommandContext(ctx, "govulncheck", "./..."), nil

	default:
		return nil, fmt.Errorf(
			"workspace command %q with arguments %v is not allowed",
			name,
			args,
		)
	}
}

func constrainedAnalyzerEnv() []string {
	return append(os.Environ(),
		"GOMAXPROCS=2",
		"GOFLAGS=-p=1",
		"GOMEMLIMIT=384MiB",
	)
}
