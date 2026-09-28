package workspace

import "context"

type Workspace interface {
	Root() string

	Run(
		ctx context.Context,
		name string,
		args ...string,
	) CommandResult

	Close() error
}

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
