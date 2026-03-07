package bootstrap_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/bootstrap"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPricingDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(wd, "..", "..", "pricing"))
}

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{Path: filepath.Join(t.TempDir(), "guardian.db")},
		Proxy: config.ProxyConfig{
			Listen:         "127.0.0.1:0",
			ReadTimeout:    "30s",
			WriteTimeout:   "60s",
			MaxBodySize:    1024,
			AddCostHeaders: true,
		},
		Pricing: config.PricingConfig{Dir: testPricingDir(t)},
		Logging: config.LoggingConfig{Level: "info", Format: "text"},
		Defaults: config.DefaultsConfig{
			Project: "default",
		},
	}

	service, err := bootstrap.NewService(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { service.Close() })

	assert.NotNil(t, service.Server)
	assert.Equal(t, cfg.Proxy.Listen, service.Server.Addr)
	assert.NotNil(t, service.Tracker)
}

func TestService_RunAndShutdown(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{Path: filepath.Join(t.TempDir(), "guardian.db")},
		Proxy: config.ProxyConfig{
			Listen:         "127.0.0.1:0",
			ReadTimeout:    "30s",
			WriteTimeout:   "60s",
			MaxBodySize:    1024,
			AddCostHeaders: true,
		},
		Pricing: config.PricingConfig{Dir: testPricingDir(t)},
		Logging: config.LoggingConfig{Level: "info", Format: "text"},
		Defaults: config.DefaultsConfig{
			Project: "default",
		},
	}

	service, err := bootstrap.NewService(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { service.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx, func(addr string) {
			addrCh <- addr
		})
	}()

	addr := <-addrCh
	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		var payload map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return false
		}
		return payload["status"] == "ok"
	}, time.Second, 25*time.Millisecond)

	cancel()
	require.NoError(t, <-errCh)
}
