package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate usage and cost reports",
	Long:  `Generate aggregated usage reports by provider, model, project, and time period.`,
	RunE:  runReport,
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringP("period", "P", "daily", "Report period (daily, weekly, monthly)")
	reportCmd.Flags().StringP("provider", "p", "", "Filter by provider")
	reportCmd.Flags().StringP("model", "m", "", "Filter by model")
	reportCmd.Flags().String("project", "", "Filter by project")
	reportCmd.Flags().Bool("detailed", false, "Show individual records")
}

func runReport(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	period, _ := cmd.Flags().GetString("period")
	providerFilter, _ := cmd.Flags().GetString("provider")
	modelFilter, _ := cmd.Flags().GetString("model")
	projectFilter, _ := cmd.Flags().GetString("project")
	detailed, _ := cmd.Flags().GetBool("detailed")

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	budgetPeriod := tracker.BudgetPeriod(period)
	start, end := tracker.PeriodBounds(budgetPeriod)

	filter := tracker.ReportFilter{
		Provider:  providerFilter,
		Model:     modelFilter,
		Project:   projectFilter,
		StartTime: start,
		EndTime:   end,
	}

	summary, err := t.Report(cmd.Context(), filter)
	if err != nil {
		return fmt.Errorf("generate report: %w", err)
	}

	fmt.Printf("=== LLM Cost Report (%s) ===\n", period)
	fmt.Printf("Period: %s to %s\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	fmt.Printf("Total Cost:          $%.4f\n", summary.TotalCostUSD)
	fmt.Printf("Total Input Tokens:  %d\n", summary.TotalInputTokens)
	fmt.Printf("Total Output Tokens: %d\n", summary.TotalOutputTokens)
	fmt.Printf("Total Requests:      %d\n", summary.RecordCount)

	if len(summary.ByProvider) > 0 {
		fmt.Printf("\nBy Provider:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  PROVIDER\tCOST\n")
		for name, cost := range summary.ByProvider {
			fmt.Fprintf(w, "  %s\t$%.4f\n", name, cost)
		}
		w.Flush()
	}

	if len(summary.ByModel) > 0 {
		fmt.Printf("\nBy Model:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  MODEL\tCOST\n")
		for name, cost := range summary.ByModel {
			fmt.Fprintf(w, "  %s\t$%.4f\n", name, cost)
		}
		w.Flush()
	}

	if detailed {
		records, err := t.Query(cmd.Context(), filter)
		if err != nil {
			return fmt.Errorf("query records: %w", err)
		}

		if len(records) > 0 {
			fmt.Printf("\nDetailed Records:\n")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  TIMESTAMP\tPROVIDER\tMODEL\tIN\tOUT\tCOST\tPROJECT\n")
			for _, r := range records {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%d\t$%.6f\t%s\n",
					r.Timestamp.Format("2006-01-02 15:04"),
					r.Provider, r.Model,
					r.InputTokens, r.OutputTokens,
					r.CostUSD, r.Project,
				)
			}
			w.Flush()
		}
	}

	return nil
}
