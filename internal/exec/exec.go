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

type FakeCmdRunner struct {
	handler  func(argv []string)
	LastArgs []string
}

func NewFakeCmdRunner(handler func(argv []string)) *FakeCmdRunner {
	return &FakeCmdRunner{handler: handler}
}

func (f *FakeCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	f.LastArgs = append([]string{cmd}, args...)
	f.handler(append([]string{cmd}, args...))
	return nil
}
