package analyzer

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type CommandRunner interface {
	Run(
		ctx context.Context,
		name string,
		args ...string,
	) CommandResult

	RunWithInput(
		ctx context.Context,
		input string,
		name string,
		args ...string,
	) CommandResult
}

type OSCommandRunner struct{}

func NewOSCommandRunner() *OSCommandRunner {
	return &OSCommandRunner{}
}

func (r *OSCommandRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) CommandResult {
	return r.run(ctx, nil, name, args...)
}

func (r *OSCommandRunner) RunWithInput(
	ctx context.Context,
	input string,
	name string,
	args ...string,
) CommandResult {
	return r.run(
		ctx,
		bytes.NewBufferString(input),
		name,
		args...,
	)
}

func (r *OSCommandRunner) run(
	ctx context.Context,
	input *bytes.Buffer,
	name string,
	args ...string,
) CommandResult {
	cmd, err := newAllowedCommand(ctx, name, args...)
	if err != nil {
		return CommandResult{
			ExitCode: -1,
			Stderr:   err.Error(),
		}
	}

	if input != nil {
		cmd.Stdin = input
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

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

func newAllowedCommand(
	ctx context.Context,
	name string,
	args ...string,
) (*exec.Cmd, error) {
	switch {
	case name == "gofmt" &&
		len(args) == 1 &&
		args[0] == "-d":
		return exec.CommandContext(ctx, "gofmt", "-d"), nil

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

	case name == "go" &&
		len(args) == 1 &&
		args[0] == "version":
		return exec.CommandContext(ctx, "go", "version"), nil

	case name == "gofmt" &&
		len(args) == 1 &&
		args[0] == "-unknown-flag":
		return exec.CommandContext(ctx, "gofmt", "-unknown-flag"), nil

	default:
		return nil, fmt.Errorf(
			"command %q with arguments %v is not allowed",
			name,
			args,
		)
	}
}
