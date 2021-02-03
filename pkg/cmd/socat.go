package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/ctlptl/internal/socat"
)

func NewSocatCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "socat",
		Short: "Use socat to connect components. Experimental.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "connect-remote-docker",
		Short:   "Connects a local port to a remote port on a machine running Docker",
		Example: "  ctlptl socat connect-remote-docker [port]\n",
		Run:     connectRemoteDocker,
		Args:    cobra.ExactArgs(1),
	})

	return cmd
}

func connectRemoteDocker(cmd *cobra.Command, args []string) {
	port, err := strconv.Atoi(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect-remote-docker: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	c, err := socat.DefaultController(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect-remote-docker: %v\n", err)
		os.Exit(1)
	}

	err = c.ConnectRemoteDockerPort(ctx, port)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect-remote-docker: %v\n", err)
		os.Exit(1)
	}
}
