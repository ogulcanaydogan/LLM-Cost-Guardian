package providers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
)

func newTestAnthropic(t *testing.T) *providers.Anthropic {
	t.Helper()
	cfg := &providers.ProviderConfig{
		Provider: "anthropic",
		Updated:  "2026-02-01",
		Models: []providers.ModelPricing{
			{Model: "claude-3.5-sonnet", InputPerMillion: 3.00, OutputPerMillion: 15.00, CachedInputPerMillion: 0.30},
			{Model: "claude-3-haiku", InputPerMillion: 0.25, OutputPerMillion: 1.25, CachedInputPerMillion: 0.03},
			{Model: "claude-3-opus", InputPerMillion: 15.00, OutputPerMillion: 75.00, CachedInputPerMillion: 1.50},
		},
	}
	return providers.NewAnthropic(cfg)
}

func TestAnthropic_Name(t *testing.T) {
	p := newTestAnthropic(t)
	assert.Equal(t, "anthropic", p.Name())
}

func TestAnthropic_Models(t *testing.T) {
	p := newTestAnthropic(t)
	models := p.Models()
	assert.Len(t, models, 3)
}

func TestAnthropic_SupportsModel(t *testing.T) {
	p := newTestAnthropic(t)
	assert.True(t, p.SupportsModel("claude-3.5-sonnet"))
	assert.True(t, p.SupportsModel("claude-3-haiku"))
	assert.False(t, p.SupportsModel("gpt-4o"))
}

func TestAnthropic_PricePerToken(t *testing.T) {
	p := newTestAnthropic(t)

	tests := []struct {
		name      string
		model     string
		tokenType providers.TokenType
		expected  float64
	}{
		{"sonnet input", "claude-3.5-sonnet", providers.TokenInput, 3.00 / 1_000_000},
		{"sonnet output", "claude-3.5-sonnet", providers.TokenOutput, 15.00 / 1_000_000},
		{"sonnet cached", "claude-3.5-sonnet", providers.TokenCachedInput, 0.30 / 1_000_000},
		{"haiku input", "claude-3-haiku", providers.TokenInput, 0.25 / 1_000_000},
		{"haiku cached", "claude-3-haiku", providers.TokenCachedInput, 0.03 / 1_000_000},
		{"opus output", "claude-3-opus", providers.TokenOutput, 75.00 / 1_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := p.PricePerToken(tt.model, tt.tokenType)
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, price, 1e-12)
		})
	}
}

func TestAnthropic_CachedInput_Fallback(t *testing.T) {
	cfg := &providers.ProviderConfig{
		Provider: "anthropic",
		Models: []providers.ModelPricing{
			{Model: "test-model", InputPerMillion: 5.00, OutputPerMillion: 10.00},
		},
	}
	p := providers.NewAnthropic(cfg)

	price, err := p.PricePerToken("test-model", providers.TokenCachedInput)
	require.NoError(t, err)
	assert.InDelta(t, 5.00/1_000_000, price, 1e-12)
}

func TestAnthropic_PricePerToken_UnknownModel(t *testing.T) {
	p := newTestAnthropic(t)
	_, err := p.PricePerToken("unknown-model", providers.TokenInput)
	assert.Error(t, err)
}

func TestAnthropic_PricePerToken_UnknownTokenType(t *testing.T) {
	p := newTestAnthropic(t)
	_, err := p.PricePerToken("claude-3.5-sonnet", providers.TokenType(99))
	assert.Error(t, err)
}
