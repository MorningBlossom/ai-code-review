package analyzer

import (
	"bytes"
	"context"
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
	cmd := exec.CommandContext(ctx, name, args...)

	if input != nil {
		cmd.Stdin = input
	}

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
