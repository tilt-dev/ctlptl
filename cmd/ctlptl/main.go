package main

import (
	"flag"
	"os"

	"github.com/spf13/pflag"
	"github.com/tilt-dev/ctlptl/pkg/cmd"
	"k8s.io/klog/v2"
)

func main() {
	command := cmd.NewRootCommand()

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
