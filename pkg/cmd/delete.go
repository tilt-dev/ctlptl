package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"github.com/tilt-dev/ctlptl/pkg/visitor"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type DeleteOptions struct {
	*genericclioptions.PrintFlags
	*genericclioptions.FileNameFlags
	genericclioptions.IOStreams

	IgnoreNotFound bool
	Filenames      []string
}

func NewDeleteOptions() *DeleteOptions {
	o := &DeleteOptions{
		PrintFlags: genericclioptions.NewPrintFlags("deleted"),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
	}
	o.FileNameFlags = &genericclioptions.FileNameFlags{Filenames: &o.Filenames}
	return o
}

func (o *DeleteOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "delete -f FILENAME",
		Short: "Delete a currently running cluster",
		Example: "  ctlptl delete -f cluster.yaml\n" +
			"  ctlptl delete cluster minikube",
		Run: o.Run,
	}

	o.FileNameFlags.AddFlags(cmd.Flags())

	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")

	return cmd
}

func (o *DeleteOptions) Run(cmd *cobra.Command, args []string) {
	cluster, err := cluster.DefaultController(o.IOStreams)
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}

	err = o.run(cluster, args)
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}
}

type clusterDeleter interface {
	Delete(ctx context.Context, name string) error
}

func (o *DeleteOptions) run(cd clusterDeleter, args []string) error {
	hasFiles := len(o.Filenames) > 0
	hasNames := len(args) >= 2
	if !(hasFiles || hasNames) {
		return fmt.Errorf("Expected resources, specified as files ('ctlptl delete -f') or names ('ctlptl delete cluster foo`)")
	}
	if hasFiles && hasNames {
		return fmt.Errorf("Can only specify one of {files, resource names}")
	}

	var resources []runtime.Object
	if hasFiles {
		visitors, err := visitor.FromStrings(o.Filenames, o.In)
		if err != nil {
			return err
		}

		resources, err = visitor.DecodeAll(visitors)
		if err != nil {
			return err
		}
	} else {
		t := args[0]
		names := args[1:]
		switch t {
		case "cluster", "clusters":
			for _, name := range names {
				resources = append(resources, &api.Cluster{
					TypeMeta: cluster.TypeMeta(),
					Name:     name,
				})
			}
		default:
			return fmt.Errorf("Unrecognized type: %s", t)
		}
	}

	ctx := context.TODO()

	printer, err := toPrinter(o.PrintFlags)
	if err != nil {
		return err
	}

	for _, resource := range resources {
		switch resource := resource.(type) {
		case *api.Cluster:
			cluster.FillDefaults(resource)
			err := cd.Delete(ctx, resource.Name)
			if err != nil {
				if o.IgnoreNotFound && errors.IsNotFound(err) {
					continue
				}
				return err
			}
			err = printer.PrintObj(resource, o.Out)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot delete: %T", resource)
		}
	}
	return nil
}
