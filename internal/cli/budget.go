package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/spf13/cobra"
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
	budgetSetCmd.Flags().String("tenant", "", "Tenant scope for this budget (default from config)")
	budgetSetCmd.Flags().String("project", "", "Project scope for this budget (empty = global)")
	budgetSetCmd.Flags().Float64P("limit", "l", 0, "Spending limit in USD")
	budgetSetCmd.Flags().StringP("period", "P", "monthly", "Budget period (daily, weekly, monthly)")
	budgetSetCmd.Flags().Float64("alert-at", 80, "Alert threshold percentage")
	budgetStatusCmd.Flags().String("tenant", "", "Show budgets for the given tenant (default from config)")
	budgetStatusCmd.Flags().String("project", "", "Show budgets applicable to the given project")
	_ = budgetSetCmd.MarkFlagRequired("limit")
}

func runBudgetSet(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	tenant, _ := cmd.Flags().GetString("tenant")
	project, _ := cmd.Flags().GetString("project")
	limit, _ := cmd.Flags().GetFloat64("limit")
	period, _ := cmd.Flags().GetString("period")
	alertAt, _ := cmd.Flags().GetFloat64("alert-at")

	if tenant == "" {
		tenant = cfg.Auth.DefaultTenant
	}

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	budget := &tracker.Budget{
		Tenant:            tenant,
		Name:              name,
		Project:           project,
		LimitUSD:          limit,
		Period:            tracker.BudgetPeriod(period),
		AlertThresholdPct: alertAt,
	}

	if err := store.SetBudget(commandContext(cmd), budget); err != nil {
		return fmt.Errorf("set budget: %w", err)
	}

	fmt.Printf("Budget set:\n")
	fmt.Printf("  Tenant:    %s\n", tenant)
	fmt.Printf("  Name:      %s\n", name)
	if project == "" {
		fmt.Printf("  Scope:     global\n")
	} else {
		fmt.Printf("  Scope:     project:%s\n", project)
	}
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

	tenantFilter, _ := cmd.Flags().GetString("tenant")
	if tenantFilter == "" {
		tenantFilter = cfg.Auth.DefaultTenant
	}

	budgets, err := store.ListBudgets(commandContext(cmd))
	if err != nil {
		return fmt.Errorf("list budgets: %w", err)
	}

	projectFilter, _ := cmd.Flags().GetString("project")
	if tenantFilter != "" || projectFilter != "" {
		var filtered []tracker.Budget
		for _, budget := range budgets {
			if tenantFilter != "" && budget.Tenant != tenantFilter {
				continue
			}
			if projectFilter == "" || budget.Project == "" || budget.Project == projectFilter {
				filtered = append(filtered, budget)
			}
		}
		budgets = filtered
	}

	if len(budgets) == 0 {
		fmt.Println("No budgets configured. Use 'lcg budget set' to create one.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tNAME\tSCOPE\tPERIOD\tLIMIT\tSPENT\tREMAINING\tUSAGE\tALERT AT\n")
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

		scope := "global"
		if b.Project != "" {
			scope = "project:" + b.Project
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t$%.2f\t$%.2f\t$%.2f\t%.1f%%%s\t%.0f%%\n",
			b.Tenant, b.Name, scope, b.Period, b.LimitUSD, b.CurrentSpend,
			remaining, pct, status, b.AlertThresholdPct,
		)
	}
	w.Flush()

	return nil
}
