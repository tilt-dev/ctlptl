package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

type GetOptions struct {
	*genericclioptions.PrintFlags
	genericclioptions.IOStreams
	StartTime      time.Time
	IgnoreNotFound bool
	FieldSelector  string
}

func NewGetOptions() *GetOptions {
	return &GetOptions{
		PrintFlags: genericclioptions.NewPrintFlags(""),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
		StartTime:  time.Now(),
	}
}

func (o *GetOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "get [type] [name]",
		Short: "Read the currently running clusters",
		Example: "  ctlptl get\n" +
			"  ctlptl get microk8s -o yaml",
		Run:  o.Run,
		Args: cobra.MaximumNArgs(2),
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")

	return cmd
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) {
	ctx := context.TODO()
	t := "cluster"
	if len(args) >= 1 {
		t = args[0]
	}
	var resources []runtime.Object
	switch t {
	case "cluster", "clusters":
		c, err := cluster.DefaultController()
		if err != nil {
			_, _ = fmt.Fprintf(o.ErrOut, "Loading controller: %v\n", err)
			os.Exit(1)
		}

		var clusters []*api.Cluster
		if len(args) >= 2 {
			cluster, err := c.Get(ctx, args[1])
			if err != nil {
				if errors.IsNotFound(err) && o.IgnoreNotFound {
					os.Exit(0)
				}
				_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
				os.Exit(1)
			}
			clusters = []*api.Cluster{cluster}
		} else {
			clusters, err = c.List(ctx, cluster.ListOptions{FieldSelector: o.FieldSelector})
			if err != nil {
				_, _ = fmt.Fprintf(o.ErrOut, "List clusters: %v\n", err)
				os.Exit(1)
			}
		}

		resources = o.clustersAsResources(clusters)
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
	if len(objs) == 0 {
		fmt.Println("No resources found")
		return nil
	}

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
			metav1.TableColumnDefinition{
				Name: "Age",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Registry",
				Type: "string",
			},
		},
	}

	for _, cluster := range clusters {
		age := "unknown"
		cTime := cluster.Status.CreationTimestamp.Time
		if !cTime.IsZero() {
			age = duration.ShortHumanDuration(o.StartTime.Sub(cTime))
		}

		rHost := ""
		if cluster.Status.LocalRegistryHosting != nil {
			rHost = cluster.Status.LocalRegistryHosting.Host
		}
		if rHost == "" {
			rHost = "none"
		}

		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []interface{}{
				cluster.Name,
				cluster.Product,
				age,
				rHost,
			},
		})
	}

	return []runtime.Object{&table}
}
