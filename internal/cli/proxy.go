package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yapay-ai/llm-cost-guardian/internal/proxy"
	"github.com/yapay-ai/llm-cost-guardian/internal/server"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage the transparent proxy",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the transparent LLM cost tracking proxy",
	RunE:  runProxyStart,
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.AddCommand(proxyStartCmd)

	proxyStartCmd.Flags().StringP("listen", "l", "", "Listen address (default from config)")
}

func runProxyStart(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	listen, _ := cmd.Flags().GetString("listen")
	if listen != "" {
		cfg.Proxy.Listen = listen
	}

	logger := newLogger(cfg)

	usageTracker, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	// Create proxy handler
	proxyHandler := proxy.NewHandler(
		usageTracker,
		cfg.Defaults.Project,
		cfg.Proxy.AddCostHeaders,
		cfg.Proxy.DenyOnExceed,
		logger,
	)

	// Create API server
	apiServer := server.NewServer(usageTracker, logger)

	// Combine routes
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

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		logger.Info("proxy started", "listen", cfg.Proxy.Listen)
		fmt.Fprintf(os.Stderr, "LLM Cost Guardian proxy listening on %s\n", cfg.Proxy.Listen)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		logger.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
	}

	logger.Info("proxy stopped")
	return nil
}
