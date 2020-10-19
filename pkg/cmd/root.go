package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "ctlptl [command]",
		Short: "Mess around with local Kubernetes clusters without consequences",
		Example: "  ctlptl get clusters\n" +
			"  ctlptl apply -f my-cluster.yaml",
	}

	rootCmd.AddCommand(NewGetOptions().Command())

	return rootCmd
}
