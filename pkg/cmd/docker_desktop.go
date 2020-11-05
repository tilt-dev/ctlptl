package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"gopkg.in/yaml.v3"
)

func NewDockerDesktopCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "docker-desktop",
		Short: "Debugging tool for the Docker Desktop client",
		Example: "  ctlptl docker-desktop settings\n" +
			"  ctlptl docker-desktop reset\n" +
			"  ctlptl docker-desktop set KEY VALUE",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "settings",
		Short: "Print the docker-desktop settings",
		Run:   withDockerDesktopClient(dockerDesktopSettings),
		Args:  cobra.ExactArgs(0),
	})

	return cmd
}

func withDockerDesktopClient(run func(client cluster.DockerForMacClient) error) func(_ *cobra.Command, args []string) {
	return func(_ *cobra.Command, args []string) {
		if runtime.GOOS != "darwin" {
			_, _ = fmt.Fprintln(os.Stderr, "ctlptl docker-desktop: currently only works on Mac")
			os.Exit(1)
		}

		c, err := cluster.NewDockerForMacClient()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ctlptl docker-desktop: %v\n", err)
			os.Exit(1)
		}

		err = run(c)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ctlptl docker-desktop: %v\n", err)
			os.Exit(1)
		}
	}
}

func dockerDesktopSettings(c cluster.DockerForMacClient) error {
	settings, err := c.SettingsValues(context.Background())
	if err != nil {
		return err
	}

	encoder := yaml.NewEncoder(os.Stdout)
	return encoder.Encode(settings)
}
