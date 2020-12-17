package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tilt-dev/ctlptl/pkg/cmd"
	"k8s.io/klog/v2"
)

// Magic variables set by goreleaser
var version string
var date string

func main() {
	cmd.Version = version

	command := cmd.NewRootCommand()
	command.AddCommand(newVersionCommand())

	klog.InitFlags(nil)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
	})

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Current ctlptl version",
		Run: func(_ *cobra.Command, args []string) {
			fmt.Println(versionStamp())
		},
	}
}

func versionStamp() string {
	timeIndex := strings.Index(date, "T")
	if timeIndex != -1 {
		date = date[0:timeIndex]
	}

	if date == "" {
		date = "unknown"
	}

	if version == "" {
		version = "0.0.0-master"
	}

	return fmt.Sprintf("v%s, built %s", version, date)
}
