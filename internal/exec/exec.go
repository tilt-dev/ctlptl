package exec

import (
	"context"
	"os/exec"
)

// A dummy package to help with mocking out exec.NewCommand

type CmdRunner interface {
	Run(ctx context.Context, cmd string, args ...string) error
}

type RealCmdRunner struct{}

func (RealCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return exec.CommandContext(ctx, cmd, args...).Run()
}

type FakeCmdRunner func(argv []string)

func (f FakeCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	f(append([]string{cmd}, args...))
	return nil
}
