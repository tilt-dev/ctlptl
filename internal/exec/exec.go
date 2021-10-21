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
	// For some reason, ExitError only gets populated with Stderr if we call Output().
	_, err := exec.CommandContext(ctx, cmd, args...).Output()
	return err
}

type FakeCmdRunner func(argv []string)

func (f FakeCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	f(append([]string{cmd}, args...))
	return nil
}
