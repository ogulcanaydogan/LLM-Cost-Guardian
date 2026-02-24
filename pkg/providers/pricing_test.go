package providers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
)

func TestLoadPricing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	data := []byte(`
provider: test
updated: "2026-01-01"
models:
  - model: test-model
    input_per_million: 1.0
    output_per_million: 2.0
`)
	err := os.WriteFile(path, data, 0o644)
	require.NoError(t, err)

	cfg, err := providers.LoadPricing(path)
	require.NoError(t, err)
	assert.Equal(t, "test", cfg.Provider)
	assert.Len(t, cfg.Models, 1)
	assert.Equal(t, "test-model", cfg.Models[0].Model)
	assert.Equal(t, 1.0, cfg.Models[0].InputPerMillion)
	assert.Equal(t, 2.0, cfg.Models[0].OutputPerMillion)
}

func TestLoadPricing_FileNotFound(t *testing.T) {
	_, err := providers.LoadPricing("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestLoadPricing_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")
	err := os.WriteFile(path, []byte("invalid: [yaml"), 0o644)
	require.NoError(t, err)

	_, err = providers.LoadPricing(path)
	assert.Error(t, err)
}

func TestLoadPricing_MissingProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noprov.yaml")
	data := []byte(`
models:
  - model: test
    input_per_million: 1.0
    output_per_million: 2.0
`)
	err := os.WriteFile(path, data, 0o644)
	require.NoError(t, err)

	_, err = providers.LoadPricing(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing provider")
}

func TestLoadPricing_NoModels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nomodels.yaml")
	data := []byte(`
provider: test
models: []
`)
	err := os.WriteFile(path, data, 0o644)
	require.NoError(t, err)

	_, err = providers.LoadPricing(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no models")
}

func TestLoadPricingFromBytes(t *testing.T) {
	data := []byte(`
provider: bytes-test
models:
  - model: m1
    input_per_million: 5.0
    output_per_million: 10.0
`)
	cfg, err := providers.LoadPricingFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "bytes-test", cfg.Provider)
	assert.Len(t, cfg.Models, 1)
}
