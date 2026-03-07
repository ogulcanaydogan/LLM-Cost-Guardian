package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/proxy"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/server"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/alerts"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

// Service wires the proxy and JSON API into a runnable HTTP server.
type Service struct {
	Config  *config.Config
	Logger  *slog.Logger
	Tracker *tracker.UsageTracker
	Store   storage.Storage
	Server  *http.Server
}

// NewLogger creates a structured logger from config.
func NewLogger(cfg *config.Config) *slog.Logger {
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

// NewRegistry creates and populates a provider registry from pricing files.
func NewRegistry(cfg *config.Config) (*providers.Registry, error) {
	registry := providers.NewRegistry()
	pricingDir := resolvePricingDir(cfg.Pricing.Dir)

	loaders := []struct {
		filename string
		load     func(string) (providers.Provider, error)
	}{
		{
			filename: "openai.yaml",
			load: func(path string) (providers.Provider, error) {
				return providers.NewOpenAIFromFile(path)
			},
		},
		{
			filename: "anthropic.yaml",
			load: func(path string) (providers.Provider, error) {
				return providers.NewAnthropicFromFile(path)
			},
		},
	}

	for _, loader := range loaders {
		path := filepath.Join(pricingDir, loader.filename)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		provider, err := loader.load(path)
		if err != nil {
			return nil, fmt.Errorf("load %s pricing: %w", loader.filename, err)
		}
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// NewStorage creates a storage backend from config.
func NewStorage(cfg *config.Config) (storage.Storage, error) {
	return storage.NewSQLite(cfg.Storage.Path)
}

// NewNotifiers creates alert notifiers from config.
func NewNotifiers(cfg *config.Config) []alerts.Notifier {
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

// NewTracker creates a fully wired usage tracker and returns the underlying store.
func NewTracker(cfg *config.Config) (*tracker.UsageTracker, storage.Storage, *slog.Logger, error) {
	logger := NewLogger(cfg)

	registry, err := NewRegistry(cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	store, err := NewStorage(cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	notifiers := NewNotifiers(cfg)
	budgetMgr := tracker.NewBudgetManager(store, notifiers, logger)
	usageTracker := tracker.NewUsageTracker(registry, store, budgetMgr, logger)

	return usageTracker, store, logger, nil
}

// NewService creates a proxy service with shared tracker, JSON API, and HTTP server wiring.
func NewService(cfg *config.Config) (*Service, error) {
	usageTracker, store, logger, err := NewTracker(cfg)
	if err != nil {
		return nil, err
	}

	proxyHandler := proxy.NewHandler(
		usageTracker,
		cfg.Defaults.Project,
		cfg.Proxy.MaxBodySize,
		cfg.Proxy.AddCostHeaders,
		cfg.Proxy.DenyOnExceed,
		logger,
	)
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

	return &Service{
		Config:  cfg,
		Logger:  logger,
		Tracker: usageTracker,
		Store:   store,
		Server: &http.Server{
			Addr:         cfg.Proxy.Listen,
			Handler:      mux,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
	}, nil
}

// Run starts the server and shuts it down when the context is canceled.
func (s *Service) Run(ctx context.Context, onStart func(addr string)) error {
	listener, err := net.Listen("tcp", s.Server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		addr := listener.Addr().String()
		s.Logger.Info("proxy started", "listen", addr)
		if onStart != nil {
			onStart(addr)
		}
		errCh <- s.Server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.Logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.Server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}

		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}

		s.Logger.Info("proxy stopped")
		return nil
	}
}

// Close releases resources owned by the service.
func (s *Service) Close() error {
	if s.Store == nil {
		return nil
	}
	return s.Store.Close()
}

func resolvePricingDir(pricingDir string) string {
	if _, err := os.Stat(pricingDir); err == nil {
		return pricingDir
	}

	exePath, err := os.Executable()
	if err != nil || exePath == "" {
		return pricingDir
	}

	altDir := filepath.Join(filepath.Dir(exePath), "pricing")
	if _, err := os.Stat(altDir); err == nil {
		return altDir
	}

	return pricingDir
}
