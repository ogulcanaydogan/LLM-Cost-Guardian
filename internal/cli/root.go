package cli

import (
	"context"
	"log/slog"
	"os"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/bootstrap"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/alerts"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "lcg",
	Short: "LLM Cost Guardian - Multi-provider LLM cost tracking and budgeting",
	Long: `LLM Cost Guardian tracks token usage and costs across multiple LLM providers.
It provides a transparent proxy for automatic tracking, CLI for manual tracking,
budget limits with alerts, and reporting capabilities.`,
	SilenceUsage: true,
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.lcg/config.yaml)")
}

// loadConfig loads the configuration.
func loadConfig() (*config.Config, error) {
	return config.Load(cfgFile)
}

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// newLogger creates a structured logger from config.
func newLogger(cfg *config.Config) *slog.Logger {
	return bootstrap.NewLogger(cfg)
}

// initRegistry creates and populates a provider registry from pricing files.
func initRegistry(cfg *config.Config) (*providers.Registry, error) {
	return bootstrap.NewRegistry(cfg)
}

// initStorage creates a storage backend from config.
func initStorage(cfg *config.Config) (storage.Storage, error) {
	return bootstrap.NewStorage(cfg)
}

// initNotifiers creates alert notifiers from config.
func initNotifiers(cfg *config.Config) []alerts.Notifier {
	return bootstrap.NewNotifiers(cfg)
}

// initTracker creates a fully wired usage tracker.
func initTracker(cfg *config.Config) (*tracker.UsageTracker, storage.Storage, error) {
	usageTracker, store, _, err := bootstrap.NewTracker(cfg)
	return usageTracker, store, err
}
