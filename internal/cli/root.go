package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yapay-ai/llm-cost-guardian/internal/config"
	"github.com/yapay-ai/llm-cost-guardian/pkg/alerts"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
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

// newLogger creates a structured logger from config.
func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var handler slog.Handler
	if cfg.Logging.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}

	return slog.New(handler)
}

// initRegistry creates and populates a provider registry from pricing files.
func initRegistry(cfg *config.Config) (*providers.Registry, error) {
	registry := providers.NewRegistry()
	pricingDir := cfg.Pricing.Dir

	// Try to find pricing directory
	if _, err := os.Stat(pricingDir); os.IsNotExist(err) {
		// Try relative to executable
		exePath, _ := os.Executable()
		if exePath != "" {
			altDir := filepath.Join(filepath.Dir(exePath), "pricing")
			if _, altErr := os.Stat(altDir); altErr == nil {
				pricingDir = altDir
			}
		}
	}

	// Load OpenAI pricing
	openaiPath := filepath.Join(pricingDir, "openai.yaml")
	if _, err := os.Stat(openaiPath); err == nil {
		p, err := providers.NewOpenAIFromFile(openaiPath)
		if err != nil {
			return nil, fmt.Errorf("load openai pricing: %w", err)
		}
		if err := registry.Register(p); err != nil {
			return nil, err
		}
	}

	// Load Anthropic pricing
	anthropicPath := filepath.Join(pricingDir, "anthropic.yaml")
	if _, err := os.Stat(anthropicPath); err == nil {
		p, err := providers.NewAnthropicFromFile(anthropicPath)
		if err != nil {
			return nil, fmt.Errorf("load anthropic pricing: %w", err)
		}
		if err := registry.Register(p); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// initStorage creates a storage backend from config.
func initStorage(cfg *config.Config) (storage.Storage, error) {
	return storage.NewSQLite(cfg.Storage.Path)
}

// initNotifiers creates alert notifiers from config.
func initNotifiers(cfg *config.Config) []alerts.Notifier {
	var notifiers []alerts.Notifier

	if cfg.Alerts.Slack.Enabled && cfg.Alerts.Slack.WebhookURL != "" {
		notifiers = append(notifiers, alerts.NewSlackNotifier(
			cfg.Alerts.Slack.WebhookURL,
			cfg.Alerts.Slack.Channel,
		))
	}

	if cfg.Alerts.Webhook.Enabled && cfg.Alerts.Webhook.URL != "" {
		notifiers = append(notifiers, alerts.NewWebhookNotifier(
			cfg.Alerts.Webhook.URL,
			cfg.Alerts.Webhook.Secret,
		))
	}

	return notifiers
}

// initTracker creates a fully wired usage tracker.
func initTracker(cfg *config.Config) (*tracker.UsageTracker, storage.Storage, error) {
	logger := newLogger(cfg)

	registry, err := initRegistry(cfg)
	if err != nil {
		return nil, nil, err
	}

	store, err := initStorage(cfg)
	if err != nil {
		return nil, nil, err
	}

	notifiers := initNotifiers(cfg)
	budgetMgr := tracker.NewBudgetManager(store, notifiers, logger)
	usageTracker := tracker.NewUsageTracker(registry, store, budgetMgr, logger)

	return usageTracker, store, nil
}
