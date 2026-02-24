package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yapay-ai/llm-cost-guardian/internal/config"
	"github.com/yapay-ai/llm-cost-guardian/internal/proxy"
	"github.com/yapay-ai/llm-cost-guardian/internal/server"
	"github.com/yapay-ai/llm-cost-guardian/pkg/alerts"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := setupLogger(cfg)

	// Initialize providers
	registry := providers.NewRegistry()
	if p, err := providers.NewOpenAIFromFile(cfg.Pricing.Dir + "/openai.yaml"); err == nil {
		_ = registry.Register(p)
	}
	if p, err := providers.NewAnthropicFromFile(cfg.Pricing.Dir + "/anthropic.yaml"); err == nil {
		_ = registry.Register(p)
	}

	// Initialize storage
	store, err := storage.NewSQLite(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer store.Close()

	// Initialize alerts
	var notifiers []alerts.Notifier
	if cfg.Alerts.Slack.Enabled {
		notifiers = append(notifiers, alerts.NewSlackNotifier(cfg.Alerts.Slack.WebhookURL, cfg.Alerts.Slack.Channel))
	}
	if cfg.Alerts.Webhook.Enabled {
		notifiers = append(notifiers, alerts.NewWebhookNotifier(cfg.Alerts.Webhook.URL, cfg.Alerts.Webhook.Secret))
	}

	// Wire up tracker
	budgetMgr := tracker.NewBudgetManager(store, notifiers, logger)
	usageTracker := tracker.NewUsageTracker(registry, store, budgetMgr, logger)

	// Create handlers
	proxyHandler := proxy.NewHandler(usageTracker, cfg.Defaults.Project, cfg.Proxy.AddCostHeaders, cfg.Proxy.DenyOnExceed, logger)
	apiServer := server.NewServer(usageTracker, logger)

	mux := http.NewServeMux()
	mux.Handle("/healthz", apiServer.Handler())
	mux.Handle("/api/", apiServer.Handler())
	mux.Handle("/", proxyHandler)

	readTimeout, _ := time.ParseDuration(cfg.Proxy.ReadTimeout)
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout, _ := time.ParseDuration(cfg.Proxy.WriteTimeout)
	if writeTimeout == 0 {
		writeTimeout = 60 * time.Second
	}

	srv := &http.Server{
		Addr:         cfg.Proxy.Listen,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("guardian started", "listen", cfg.Proxy.Listen)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		logger.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func setupLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
