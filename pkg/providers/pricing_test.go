package providers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewProviderFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vertex-ai.yaml")
	data := []byte(`
provider: vertex-ai
models:
  - model: gemini-1.5-pro
    input_per_million: 1.25
    output_per_million: 5.00
`)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	provider, err := providers.NewProviderFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, "vertex-ai", provider.Name())
	assert.True(t, provider.SupportsModel("gemini-1.5-pro"))
}

func TestTypedProvidersFromFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		data     string
		load     func(string) (providers.Provider, error)
	}{
		{
			name:     "openai",
			filename: "openai.yaml",
			data: `
provider: openai
models:
  - model: gpt-4o
    input_per_million: 2.50
    output_per_million: 10.00
`,
			load: func(path string) (providers.Provider, error) { return providers.NewOpenAIFromFile(path) },
		},
		{
			name:     "anthropic",
			filename: "anthropic.yaml",
			data: `
provider: anthropic
models:
  - model: claude-3.5-sonnet
    input_per_million: 3.00
    output_per_million: 15.00
`,
			load: func(path string) (providers.Provider, error) { return providers.NewAnthropicFromFile(path) },
		},
		{
			name:     "azure-openai",
			filename: "azure-openai.yaml",
			data: `
provider: azure-openai
models:
  - model: gpt-4o
    input_per_million: 2.50
    output_per_million: 10.00
`,
			load: func(path string) (providers.Provider, error) { return providers.NewAzureOpenAIFromFile(path) },
		},
		{
			name:     "bedrock",
			filename: "bedrock.yaml",
			data: `
provider: bedrock
models:
  - model: anthropic.claude-3-5-sonnet-20241022-v2:0
    input_per_million: 3.00
    output_per_million: 15.00
`,
			load: func(path string) (providers.Provider, error) { return providers.NewBedrockFromFile(path) },
		},
		{
			name:     "vertex-ai",
			filename: "vertex-ai.yaml",
			data: `
provider: vertex-ai
models:
  - model: gemini-1.5-pro
    input_per_million: 1.25
    output_per_million: 5.00
`,
			load: func(path string) (providers.Provider, error) { return providers.NewVertexAIFromFile(path) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.filename)
			require.NoError(t, os.WriteFile(path, []byte(tt.data), 0o644))

			provider, err := tt.load(path)
			require.NoError(t, err)
			assert.Equal(t, tt.name, provider.Name())
		})
	}
}
