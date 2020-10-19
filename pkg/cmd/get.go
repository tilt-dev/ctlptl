package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/ctlptl/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

type GetOptions struct {
	*genericclioptions.PrintFlags
	genericclioptions.IOStreams
}

func NewGetOptions() *GetOptions {
	return &GetOptions{
		PrintFlags: genericclioptions.NewPrintFlags(""),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
	}
}

func (o *GetOptions) Command() *cobra.Command {
	var getCmd = &cobra.Command{
		Use:   "get [type]",
		Short: "Read the currently running clusters",
		Example: "  ctlptl get\n" +
			"  ctlptl get -o yaml",
		Run:  o.Run,
		Args: cobra.MaximumNArgs(1),
	}

	o.PrintFlags.AddFlags(getCmd)

	return getCmd
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) {
	t := "cluster"
	if len(args) >= 1 {
		t = args[0]
	}
	var resources []runtime.Object
	switch t {
	case "cluster", "clusters":
		resources = o.clustersAsResources(o.clusters())
	default:
		_, _ = fmt.Fprintf(o.ErrOut, "Unrecognized type: %s\n", t)
		os.Exit(1)
	}

	err := o.Print(resources)
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "Error: %s\n", err)
		os.Exit(1)
	}
}

func (o *GetOptions) ToPrinter() (printers.ResourcePrinter, error) {
	if !o.OutputFlagSpecified() {
		return printers.NewTablePrinter(printers.PrintOptions{}), nil
	}
	return o.PrintFlags.ToPrinter()
}

func (o *GetOptions) Print(objs []runtime.Object) error {
	printer, err := o.ToPrinter()
	if err != nil {
		return err
	}

	for _, obj := range objs {
		err = printer.PrintObj(obj, o.Out)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *GetOptions) OutputFlagSpecified() bool {
	return o.PrintFlags.OutputFlagSpecified != nil && o.PrintFlags.OutputFlagSpecified()
}

func (o *GetOptions) clusters() []*api.Cluster {
	m := api.TypeMeta{Kind: "Cluster", APIVersion: "ctlptl.dev/v1alpha1"}
	return []*api.Cluster{
		&api.Cluster{TypeMeta: m, Name: "microk8s", Product: "microk8s"},
		&api.Cluster{TypeMeta: m, Name: "kind-kind", Product: "KIND"},
	}
}

func (o *GetOptions) clustersAsResources(clusters []*api.Cluster) []runtime.Object {
	if o.OutputFlagSpecified() {
		result := []runtime.Object{}
		for _, cluster := range clusters {
			result = append(result, cluster)
		}
		return result
	}

	table := metav1.Table{
		TypeMeta: metav1.TypeMeta{Kind: "Table", APIVersion: "metav1.k8s.io"},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			metav1.TableColumnDefinition{
				Name: "Name",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Product",
				Type: "string",
			},
		},
	}

	for _, cluster := range clusters {
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []interface{}{
				cluster.Name,
				cluster.Product,
			},
		})
	}

	return []runtime.Object{&table}
}
