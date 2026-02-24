package tracker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
)

func newTestRegistry(t *testing.T) *providers.Registry {
	t.Helper()
	r := providers.NewRegistry()

	openai := providers.NewOpenAI(&providers.ProviderConfig{
		Provider: "openai",
		Models: []providers.ModelPricing{
			{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00},
			{Model: "gpt-4o-mini", InputPerMillion: 0.15, OutputPerMillion: 0.60},
		},
	})
	_ = r.Register(openai)

	anthropic := providers.NewAnthropic(&providers.ProviderConfig{
		Provider: "anthropic",
		Models: []providers.ModelPricing{
			{Model: "claude-3.5-sonnet", InputPerMillion: 3.00, OutputPerMillion: 15.00, CachedInputPerMillion: 0.30},
		},
	})
	_ = r.Register(anthropic)

	return r
}

func TestCalculateCost(t *testing.T) {
	registry := newTestRegistry(t)

	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int64
		outputTokens int64
		expected     float64
	}{
		{
			name: "gpt-4o 1M tokens",
			provider: "openai", model: "gpt-4o",
			inputTokens: 1_000_000, outputTokens: 1_000_000,
			expected: 2.50 + 10.00,
		},
		{
			name: "gpt-4o-mini small",
			provider: "openai", model: "gpt-4o-mini",
			inputTokens: 1000, outputTokens: 500,
			expected: (0.15 * 1000 / 1_000_000) + (0.60 * 500 / 1_000_000),
		},
		{
			name: "claude-3.5-sonnet",
			provider: "anthropic", model: "claude-3.5-sonnet",
			inputTokens: 10000, outputTokens: 2000,
			expected: (3.00 * 10000 / 1_000_000) + (15.00 * 2000 / 1_000_000),
		},
		{
			name: "zero tokens",
			provider: "openai", model: "gpt-4o",
			inputTokens: 0, outputTokens: 0,
			expected: 0,
		},
	}

	calc := tracker.NewCostCalculator(registry)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := calc.Calculate(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, cost, 1e-10)
		})
	}
}

func TestCalculateCost_UnknownProvider(t *testing.T) {
	registry := newTestRegistry(t)
	calc := tracker.NewCostCalculator(registry)

	_, err := calc.Calculate("unknown", "model", 100, 100)
	assert.Error(t, err)
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	registry := newTestRegistry(t)
	calc := tracker.NewCostCalculator(registry)

	_, err := calc.Calculate("openai", "nonexistent", 100, 100)
	assert.Error(t, err)
}

func TestCalculateCostWithCache(t *testing.T) {
	cfg := &providers.ProviderConfig{
		Provider: "anthropic",
		Models: []providers.ModelPricing{
			{Model: "claude-3.5-sonnet", InputPerMillion: 3.00, OutputPerMillion: 15.00, CachedInputPerMillion: 0.30},
		},
	}
	p := providers.NewAnthropic(cfg)

	cost, err := tracker.CalculateCostWithCache(p, "claude-3.5-sonnet", 5000, 3000, 2000)
	require.NoError(t, err)

	expected := (3.00*5000 + 0.30*3000 + 15.00*2000) / 1_000_000
	assert.InDelta(t, expected, cost, 1e-10)
}

func BenchmarkCalculateCost(b *testing.B) {
	registry := providers.NewRegistry()
	openai := providers.NewOpenAI(&providers.ProviderConfig{
		Provider: "openai",
		Models:   []providers.ModelPricing{{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00}},
	})
	_ = registry.Register(openai)
	calc := tracker.NewCostCalculator(registry)

	for b.Loop() {
		_, _ = calc.Calculate("openai", "gpt-4o", 1000, 500)
	}
}
