package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"github.com/tilt-dev/ctlptl/pkg/visitor"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type ApplyOptions struct {
	*genericclioptions.PrintFlags
	*genericclioptions.FileNameFlags
	genericclioptions.IOStreams

	Filenames []string
}

func NewApplyOptions() *ApplyOptions {
	o := &ApplyOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created"),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
	}
	o.FileNameFlags = &genericclioptions.FileNameFlags{Filenames: &o.Filenames}
	return o
}

func (o *ApplyOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "apply -f FILENAME",
		Short: "Apply a cluster config to the currently running clusters",
		Example: "  ctlptl apply -f cluster.yaml\n" +
			"  cat cluster.yaml | ctlptl apply -f -",
		Run: o.Run,
	}

	o.FileNameFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *ApplyOptions) Run(cmd *cobra.Command, args []string) {
	if len(o.Filenames) == 0 {
		fmt.Fprintf(o.ErrOut, "Expected source files with -f")
		os.Exit(1)
	}

	err := o.run()
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}
}

func (o *ApplyOptions) run() error {
	ctx := context.TODO()
	c, err := cluster.DefaultController()

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	visitors, err := o.visitors()
	if err != nil {
		return err
	}

	objects, err := visitor.DecodeAll(visitors)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		cluster := obj.(*api.Cluster)
		newObj, err := c.Apply(ctx, cluster)
		if err != nil {
			return err
		}

		err = printer.PrintObj(newObj, o.Out)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *ApplyOptions) visitors() ([]visitor.Interface, error) {
	result := []visitor.Interface{}
	for _, f := range o.Filenames {

		switch {
		case f == "-":
			result = append(result, visitor.Stdin(o.In))

		case strings.Index(f, "http://") == 0 || strings.Index(f, "https://") == 0:
			url, err := url.Parse(f)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid URL %s", url)
			}
			result = append(result, visitor.URL(http.DefaultClient, f))

		default:
			result = append(result, visitor.File(f))

		}
	}
	return result, nil
}
