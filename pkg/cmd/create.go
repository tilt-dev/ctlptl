package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type CreateOptions struct {
	genericclioptions.IOStreams
}

func NewCreateOptions() *CreateOptions {
	o := &CreateOptions{
		IOStreams: genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
	}
	return o
}

func (o *CreateOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "create [cluster|registry]",
		Short: "Create a cluster or registry",
		Example: "  ctlptl create cluster docker-desktop\n" +
			"  ctlptl create cluster kind --registry=ctlptl-registry",
		Run: o.Run,
	}

	cmd.AddCommand(NewCreateClusterOptions().Command())
	cmd.AddCommand(NewCreateRegistryOptions().Command())

	return cmd
}

func (o *CreateOptions) Run(cmd *cobra.Command, args []string) {
	_, _ = fmt.Fprintln(o.IOStreams.ErrOut, cmd.Help().Error())
	os.Exit(1)
}
