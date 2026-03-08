package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/spf13/cobra"
)

var anomaliesCmd = &cobra.Command{
	Use:   "anomalies",
	Short: "List spend anomalies",
	RunE:  runAnomalies,
}

var forecastCmd = &cobra.Command{
	Use:   "forecast",
	Short: "Forecast tenant spend",
	RunE:  runForecast,
}

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend lower-cost models",
	RunE:  runRecommend,
}

var promptsCmd = &cobra.Command{
	Use:   "prompts",
	Short: "Prompt optimization insights",
}

var promptsOptimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Suggest prompt optimizations",
	RunE:  runPromptOptimize,
}

func init() {
	rootCmd.AddCommand(anomaliesCmd)
	rootCmd.AddCommand(forecastCmd)
	rootCmd.AddCommand(recommendCmd)
	rootCmd.AddCommand(promptsCmd)
	promptsCmd.AddCommand(promptsOptimizeCmd)

	for _, cmd := range []*cobra.Command{anomaliesCmd, forecastCmd, recommendCmd, promptsOptimizeCmd} {
		cmd.Flags().String("tenant", "", "Filter by tenant (default from config)")
		cmd.Flags().String("project", "", "Filter by project")
		cmd.Flags().String("provider", "", "Filter by provider")
		cmd.Flags().String("model", "", "Filter by model")
	}
}

func runAnomalies(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	filter, err := intelligenceFilter(cmd, cfg)
	if err != nil {
		return err
	}

	anomalies, err := t.DetectAnomalies(commandContext(cmd), filter)
	if err != nil {
		return err
	}

	if len(anomalies) == 0 {
		fmt.Println("No anomalies detected.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tPROJECT\tPROVIDER\tMODEL\tSEVERITY\tOBSERVED\tBASELINE\tZSCORE\n")
	for _, anomaly := range anomalies {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t$%.4f\t$%.4f\t%.2f\n",
			anomaly.Tenant, anomaly.Project, anomaly.Provider, anomaly.Model,
			anomaly.Severity, anomaly.ObservedCostUSD, anomaly.BaselineCostUSD, anomaly.ZScore,
		)
	}
	w.Flush()
	return nil
}

func runForecast(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	filter, err := intelligenceFilter(cmd, cfg)
	if err != nil {
		return err
	}

	forecasts, err := t.Forecast(commandContext(cmd), filter)
	if err != nil {
		return err
	}

	if len(forecasts) == 0 {
		fmt.Println("No forecast data available.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tPROJECT\tHORIZON\tFORECAST\tAVG DAILY\tTREND\tCONFIDENCE\n")
	for _, forecast := range forecasts {
		fmt.Fprintf(w, "%s\t%s\t%dd\t$%.4f\t$%.4f\t$%.4f\t%s\n",
			forecast.Tenant, forecast.Project, forecast.HorizonDays, forecast.ForecastCostUSD,
			forecast.AverageDailyCostUSD, forecast.TrendDailyDeltaUSD, forecast.Confidence,
		)
	}
	w.Flush()
	return nil
}

func runRecommend(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	filter, err := intelligenceFilter(cmd, cfg)
	if err != nil {
		return err
	}

	recommendations, err := t.RecommendModels(commandContext(cmd), filter)
	if err != nil {
		return err
	}

	if len(recommendations) == 0 {
		fmt.Println("No recommendations available.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tPROJECT\tCURRENT\tSUGGESTED\tSAVINGS\tSAVINGS%%\n")
	for _, recommendation := range recommendations {
		fmt.Fprintf(w, "%s\t%s\t%s/%s\t%s/%s\t$%.4f\t%.1f%%\n",
			recommendation.Tenant, recommendation.Project,
			recommendation.CurrentProvider, recommendation.CurrentModel,
			recommendation.SuggestedProvider, recommendation.SuggestedModel,
			recommendation.EstimatedSavingsUSD, recommendation.EstimatedSavingsPct,
		)
	}
	w.Flush()
	return nil
}

func runPromptOptimize(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	filter, err := intelligenceFilter(cmd, cfg)
	if err != nil {
		return err
	}

	optimizations, err := t.PromptOptimizations(commandContext(cmd), filter)
	if err != nil {
		return err
	}

	if len(optimizations) == 0 {
		fmt.Println("No prompt optimization opportunities found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tPROJECT\tPROVIDER\tMODEL\tSEVERITY\tSUGGESTION\tIMPACT\n")
	for _, optimization := range optimizations {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			optimization.Tenant, optimization.Project, optimization.Provider, optimization.Model,
			optimization.Severity, optimization.Suggestion, optimization.EstimatedImpact,
		)
	}
	w.Flush()
	return nil
}

func intelligenceFilter(cmd *cobra.Command, cfg *config.Config) (tracker.ReportFilter, error) {
	tenant, _ := cmd.Flags().GetString("tenant")
	if tenant == "" {
		tenant = cfg.Auth.DefaultTenant
	}
	project, _ := cmd.Flags().GetString("project")
	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	return tracker.ReportFilter{
		Tenant:   tenant,
		Project:  project,
		Provider: provider,
		Model:    model,
	}, nil
}
