package providers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
)

func newTestOpenAI(t *testing.T) *providers.OpenAI {
	t.Helper()
	cfg := &providers.ProviderConfig{
		Provider: "openai",
		Updated:  "2026-02-01",
		Models: []providers.ModelPricing{
			{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00},
			{Model: "gpt-4o-mini", InputPerMillion: 0.15, OutputPerMillion: 0.60},
			{Model: "gpt-4", InputPerMillion: 30.00, OutputPerMillion: 60.00},
		},
	}
	return providers.NewOpenAI(cfg)
}

func TestOpenAI_Name(t *testing.T) {
	p := newTestOpenAI(t)
	assert.Equal(t, "openai", p.Name())
}

func TestOpenAI_Models(t *testing.T) {
	p := newTestOpenAI(t)
	models := p.Models()
	assert.Len(t, models, 3)
}

func TestOpenAI_SupportsModel(t *testing.T) {
	p := newTestOpenAI(t)
	assert.True(t, p.SupportsModel("gpt-4o"))
	assert.True(t, p.SupportsModel("gpt-4o-mini"))
	assert.False(t, p.SupportsModel("nonexistent"))
}

func TestOpenAI_PricePerToken(t *testing.T) {
	p := newTestOpenAI(t)

	tests := []struct {
		name      string
		model     string
		tokenType providers.TokenType
		expected  float64
	}{
		{"gpt-4o input", "gpt-4o", providers.TokenInput, 2.50 / 1_000_000},
		{"gpt-4o output", "gpt-4o", providers.TokenOutput, 10.00 / 1_000_000},
		{"gpt-4o-mini input", "gpt-4o-mini", providers.TokenInput, 0.15 / 1_000_000},
		{"gpt-4o-mini output", "gpt-4o-mini", providers.TokenOutput, 0.60 / 1_000_000},
		{"gpt-4 input", "gpt-4", providers.TokenInput, 30.00 / 1_000_000},
		{"gpt-4o cached input", "gpt-4o", providers.TokenCachedInput, 2.50 / 1_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := p.PricePerToken(tt.model, tt.tokenType)
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, price, 1e-12)
		})
	}
}

func TestOpenAI_PricePerToken_UnknownModel(t *testing.T) {
	p := newTestOpenAI(t)
	_, err := p.PricePerToken("unknown-model", providers.TokenInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown model")
}

func TestOpenAI_PricePerToken_UnknownTokenType(t *testing.T) {
	p := newTestOpenAI(t)
	_, err := p.PricePerToken("gpt-4o", providers.TokenType(99))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown token type")
}
