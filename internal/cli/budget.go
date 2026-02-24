package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage spending budgets",
}

var budgetSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a budget",
	RunE:  runBudgetSet,
}

var budgetStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current budget status",
	RunE:  runBudgetStatus,
}

func init() {
	rootCmd.AddCommand(budgetCmd)
	budgetCmd.AddCommand(budgetSetCmd)
	budgetCmd.AddCommand(budgetStatusCmd)

	budgetSetCmd.Flags().StringP("name", "n", "default", "Budget name")
	budgetSetCmd.Flags().Float64P("limit", "l", 0, "Spending limit in USD")
	budgetSetCmd.Flags().StringP("period", "P", "monthly", "Budget period (daily, weekly, monthly)")
	budgetSetCmd.Flags().Float64("alert-at", 80, "Alert threshold percentage")
	_ = budgetSetCmd.MarkFlagRequired("limit")
}

func runBudgetSet(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	limit, _ := cmd.Flags().GetFloat64("limit")
	period, _ := cmd.Flags().GetString("period")
	alertAt, _ := cmd.Flags().GetFloat64("alert-at")

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	budget := &tracker.Budget{
		Name:              name,
		LimitUSD:          limit,
		Period:            tracker.BudgetPeriod(period),
		AlertThresholdPct: alertAt,
	}

	if err := store.SetBudget(cmd.Context(), budget); err != nil {
		return fmt.Errorf("set budget: %w", err)
	}

	fmt.Printf("Budget set:\n")
	fmt.Printf("  Name:      %s\n", name)
	fmt.Printf("  Limit:     $%.2f\n", limit)
	fmt.Printf("  Period:    %s\n", period)
	fmt.Printf("  Alert at:  %.0f%%\n", alertAt)

	return nil
}

func runBudgetStatus(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	budgets, err := store.ListBudgets(cmd.Context())
	if err != nil {
		return fmt.Errorf("list budgets: %w", err)
	}

	if len(budgets) == 0 {
		fmt.Println("No budgets configured. Use 'lcg budget set' to create one.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tPERIOD\tLIMIT\tSPENT\tREMAINING\tUSAGE\tALERT AT\n")
	for _, b := range budgets {
		remaining := b.LimitUSD - b.CurrentSpend
		if remaining < 0 {
			remaining = 0
		}
		pct := float64(0)
		if b.LimitUSD > 0 {
			pct = (b.CurrentSpend / b.LimitUSD) * 100
		}

		status := ""
		switch {
		case pct >= 100:
			status = " [EXCEEDED]"
		case pct >= 95:
			status = " [CRITICAL]"
		case pct >= b.AlertThresholdPct:
			status = " [WARNING]"
		}

		fmt.Fprintf(w, "%s\t%s\t$%.2f\t$%.2f\t$%.2f\t%.1f%%%s\t%.0f%%\n",
			b.Name, b.Period, b.LimitUSD, b.CurrentSpend,
			remaining, pct, status, b.AlertThresholdPct,
		)
	}
	w.Flush()

	return nil
}
