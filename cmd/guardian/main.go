package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/bootstrap"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
)

var runMain = func() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return run(ctx, "")
}

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfgFile string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	service, err := bootstrap.NewService(cfg)
	if err != nil {
		return err
	}
	defer service.Close()

	return service.Run(ctx, nil)
}
