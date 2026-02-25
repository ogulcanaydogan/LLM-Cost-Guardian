package tracker

import (
	"fmt"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
)

// CostCalculator computes costs for LLM API usage.
type CostCalculator struct {
	registry *providers.Registry
}

// NewCostCalculator creates a cost calculator backed by a provider registry.
func NewCostCalculator(registry *providers.Registry) *CostCalculator {
	return &CostCalculator{registry: registry}
}

// Calculate computes the USD cost for a given API call.
func (c *CostCalculator) Calculate(providerName, model string, inputTokens, outputTokens int64) (float64, error) {
	p, err := c.registry.Get(providerName)
	if err != nil {
		return 0, fmt.Errorf("cost calculation: %w", err)
	}
	return CalculateCost(p, model, inputTokens, outputTokens)
}

// CalculateCost computes the USD cost for a given API call using a provider directly.
func CalculateCost(p providers.Provider, model string, inputTokens, outputTokens int64) (float64, error) {
	inputPrice, err := p.PricePerToken(model, providers.TokenInput)
	if err != nil {
		return 0, fmt.Errorf("input pricing: %w", err)
	}

	outputPrice, err := p.PricePerToken(model, providers.TokenOutput)
	if err != nil {
		return 0, fmt.Errorf("output pricing: %w", err)
	}

	cost := float64(inputTokens)*inputPrice + float64(outputTokens)*outputPrice
	return cost, nil
}

// CalculateCostWithCache computes cost including cached input tokens (e.g., Anthropic).
func CalculateCostWithCache(p providers.Provider, model string, inputTokens, cachedInputTokens, outputTokens int64) (float64, error) {
	inputPrice, err := p.PricePerToken(model, providers.TokenInput)
	if err != nil {
		return 0, fmt.Errorf("input pricing: %w", err)
	}

	cachedPrice, err := p.PricePerToken(model, providers.TokenCachedInput)
	if err != nil {
		return 0, fmt.Errorf("cached input pricing: %w", err)
	}

	outputPrice, err := p.PricePerToken(model, providers.TokenOutput)
	if err != nil {
		return 0, fmt.Errorf("output pricing: %w", err)
	}

	cost := float64(inputTokens)*inputPrice +
		float64(cachedInputTokens)*cachedPrice +
		float64(outputTokens)*outputPrice
	return cost, nil
}
