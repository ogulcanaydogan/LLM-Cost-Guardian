package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var trackCmd = &cobra.Command{
	Use:   "track",
	Short: "Record LLM API usage manually",
	Long:  `Record a single LLM API call with provider, model, and token counts.`,
	RunE:  runTrack,
}

func init() {
	rootCmd.AddCommand(trackCmd)
	trackCmd.Flags().StringP("provider", "p", "", "LLM provider (e.g., openai, anthropic)")
	trackCmd.Flags().StringP("model", "m", "", "Model name (e.g., gpt-4o, claude-3.5-sonnet)")
	trackCmd.Flags().Int64("input-tokens", 0, "Number of input tokens")
	trackCmd.Flags().Int64("output-tokens", 0, "Number of output tokens")
	trackCmd.Flags().String("project", "", "Project name (default from config)")
	_ = trackCmd.MarkFlagRequired("provider")
	_ = trackCmd.MarkFlagRequired("model")
}

func runTrack(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	inputTokens, _ := cmd.Flags().GetInt64("input-tokens")
	outputTokens, _ := cmd.Flags().GetInt64("output-tokens")
	project, _ := cmd.Flags().GetString("project")

	if project == "" {
		project = cfg.Defaults.Project
	}

	t, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	record, err := t.Track(cmd.Context(), provider, model, inputTokens, outputTokens, project)
	if err != nil {
		return fmt.Errorf("track usage: %w", err)
	}

	fmt.Printf("Recorded usage:\n")
	fmt.Printf("  ID:            %s\n", record.ID)
	fmt.Printf("  Provider:      %s\n", record.Provider)
	fmt.Printf("  Model:         %s\n", record.Model)
	fmt.Printf("  Input tokens:  %d\n", record.InputTokens)
	fmt.Printf("  Output tokens: %d\n", record.OutputTokens)
	fmt.Printf("  Cost:          $%.6f\n", record.CostUSD)
	fmt.Printf("  Project:       %s\n", record.Project)

	return nil
}
