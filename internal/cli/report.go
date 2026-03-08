package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/reporting"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/spf13/cobra"
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
	reportCmd.Flags().String("tenant", "", "Filter by tenant (default from config)")
	reportCmd.Flags().StringP("provider", "p", "", "Filter by provider")
	reportCmd.Flags().StringP("model", "m", "", "Filter by model")
	reportCmd.Flags().String("project", "", "Filter by project")
	reportCmd.Flags().Bool("detailed", false, "Show individual records")
	reportCmd.Flags().String("format", "text", "Output format (text, csv, pdf)")
	reportCmd.Flags().String("output", "", "Output file path for csv/pdf exports")
}

func runReport(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	period, _ := cmd.Flags().GetString("period")
	tenantFilter, _ := cmd.Flags().GetString("tenant")
	providerFilter, _ := cmd.Flags().GetString("provider")
	modelFilter, _ := cmd.Flags().GetString("model")
	projectFilter, _ := cmd.Flags().GetString("project")
	detailed, _ := cmd.Flags().GetBool("detailed")
	format, _ := cmd.Flags().GetString("format")
	outputPath, _ := cmd.Flags().GetString("output")

	if tenantFilter == "" {
		tenantFilter = cfg.Auth.DefaultTenant
	}

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	budgetPeriod := tracker.BudgetPeriod(period)
	start, end := tracker.PeriodBounds(budgetPeriod)

	filter := tracker.ReportFilter{
		Tenant:    tenantFilter,
		Provider:  providerFilter,
		Model:     modelFilter,
		Project:   projectFilter,
		StartTime: start,
		EndTime:   end,
	}

	summary, err := t.Report(commandContext(cmd), filter)
	if err != nil {
		return fmt.Errorf("generate report: %w", err)
	}

	requiresRecords := detailed || format != "text"
	var records []tracker.UsageRecord
	if requiresRecords {
		records, err = t.Query(commandContext(cmd), filter)
		if err != nil {
			return fmt.Errorf("query records: %w", err)
		}
	}

	if format == "text" {
		printTextSummary(summary, period, start, end)
		printCostMap("Provider", summary.ByProvider)
		printCostMap("Model", summary.ByModel)
		printCostMap("Project", summary.ByProject)
		if detailed {
			printDetailedRecords(records)
		}
		return nil
	}

	if outputPath == "" {
		outputPath = reporting.DefaultOutputPath(period, format)
	}
	outputPath = filepath.Clean(outputPath)

	doc := reporting.ReportDocument{
		Period:      period,
		Start:       start,
		End:         end,
		Filter:      filter,
		Summary:     summary,
		Records:     records,
		Chargebacks: reporting.BuildProjectChargebacks(records),
	}

	switch format {
	case "csv":
		if err := reporting.WriteCSV(outputPath, doc, detailed); err != nil {
			return err
		}
	case "pdf":
		if err := reporting.WritePDF(outputPath, doc, detailed); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format %q", format)
	}

	fmt.Printf("Exported %s report to %s\n", format, outputPath)
	return nil
}

func printTextSummary(summary *tracker.UsageSummary, period string, start, end time.Time) {
	fmt.Printf("=== LLM Cost Report (%s) ===\n", period)
	fmt.Printf("Period: %s to %s\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	fmt.Printf("Total Cost:          $%.4f\n", summary.TotalCostUSD)
	fmt.Printf("Total Input Tokens:  %d\n", summary.TotalInputTokens)
	fmt.Printf("Total Output Tokens: %d\n", summary.TotalOutputTokens)
	fmt.Printf("Total Requests:      %d\n", summary.RecordCount)
}

func printCostMap(title string, values map[string]float64) {
	if len(values) == 0 {
		return
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("\nBy %s:\n", title)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  %s\tCOST\n", strings.ToUpper(title))
	for _, name := range names {
		fmt.Fprintf(w, "  %s\t$%.4f\n", name, values[name])
	}
	w.Flush()
}

func printDetailedRecords(records []tracker.UsageRecord) {
	if len(records) == 0 {
		return
	}

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
