package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/bootstrap"
	"github.com/spf13/cobra"
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

	service, err := bootstrap.NewService(cfg)
	if err != nil {
		return err
	}
	defer service.Close()

	ctx, stop := signal.NotifyContext(commandContext(cmd), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return service.Run(ctx, func(addr string) {
		fmt.Fprintf(os.Stderr, "LLM Cost Guardian proxy listening on %s\n", addr)
	})
}
