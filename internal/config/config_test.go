package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.Proxy.Listen)
	assert.Equal(t, "30s", cfg.Proxy.ReadTimeout)
	assert.Equal(t, "60s", cfg.Proxy.WriteTimeout)
	assert.True(t, cfg.Proxy.AddCostHeaders)
	assert.False(t, cfg.Proxy.DenyOnExceed)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "default", cfg.Defaults.Project)
	assert.Equal(t, "pricing/", cfg.Pricing.Dir)
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	data := []byte(`
storage:
  path: /tmp/test.db
proxy:
  listen: ":9090"
  deny_on_exceed: true
logging:
  level: debug
defaults:
  project: my-project
`)
	err := os.WriteFile(cfgPath, data, 0o644)
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/test.db", cfg.Storage.Path)
	assert.Equal(t, ":9090", cfg.Proxy.Listen)
	assert.True(t, cfg.Proxy.DenyOnExceed)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "my-project", cfg.Defaults.Project)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("LCG_LOGGING_LEVEL", "error")
	t.Setenv("LCG_PROXY_LISTEN", ":7070")

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, "error", cfg.Logging.Level)
	assert.Equal(t, ":7070", cfg.Proxy.Listen)
}

func TestLoad_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	err := os.WriteFile(cfgPath, []byte("invalid: [yaml"), 0o644)
	require.NoError(t, err)

	_, err = config.Load(cfgPath)
	assert.Error(t, err)
}
