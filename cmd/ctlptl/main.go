package main

import (
	"os"

	"github.com/tilt-dev/ctlptl/pkg/cmd"
)

func main() {
	command := cmd.NewRootCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
