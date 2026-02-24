package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage LLM providers",
}

var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all providers and their model pricing",
	RunE:  runProvidersList,
}

func init() {
	rootCmd.AddCommand(providersCmd)
	providersCmd.AddCommand(providersListCmd)
}

func runProvidersList(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	registry, err := initRegistry(cfg)
	if err != nil {
		return err
	}

	allProviders := registry.All()
	if len(allProviders) == 0 {
		fmt.Println("No providers configured. Check pricing directory in config.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "PROVIDER\tMODEL\tINPUT ($/1M)\tOUTPUT ($/1M)\tCACHED INPUT ($/1M)\n")

	for _, p := range allProviders {
		for _, m := range p.Models() {
			cached := "-"
			if m.CachedInputPerMillion > 0 {
				cached = fmt.Sprintf("$%.2f", m.CachedInputPerMillion)
			}
			fmt.Fprintf(w, "%s\t%s\t$%.2f\t$%.2f\t%s\n",
				p.Name(), m.Model,
				m.InputPerMillion, m.OutputPerMillion,
				cached,
			)
		}
	}
	w.Flush()

	return nil
}
