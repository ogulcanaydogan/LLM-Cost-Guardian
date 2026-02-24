package providers

import "fmt"

// Anthropic implements the Provider interface for Anthropic models.
type Anthropic struct {
	config *ProviderConfig
	models map[string]ModelPricing
}

// NewAnthropic creates a new Anthropic provider from a pricing config.
func NewAnthropic(cfg *ProviderConfig) *Anthropic {
	m := make(map[string]ModelPricing, len(cfg.Models))
	for _, model := range cfg.Models {
		m[model.Model] = model
	}
	return &Anthropic{config: cfg, models: m}
}

// NewAnthropicFromFile creates a new Anthropic provider from a YAML pricing file.
func NewAnthropicFromFile(path string) (*Anthropic, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewAnthropic(cfg), nil
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Models() []ModelPricing {
	return a.config.Models
}

func (a *Anthropic) PricePerToken(model string, tokenType TokenType) (float64, error) {
	pricing, ok := a.models[model]
	if !ok {
		return 0, fmt.Errorf("anthropic: unknown model %q", model)
	}

	switch tokenType {
	case TokenInput:
		return pricing.InputPerMillion / 1_000_000, nil
	case TokenOutput:
		return pricing.OutputPerMillion / 1_000_000, nil
	case TokenCachedInput:
		if pricing.CachedInputPerMillion > 0 {
			return pricing.CachedInputPerMillion / 1_000_000, nil
		}
		return pricing.InputPerMillion / 1_000_000, nil
	default:
		return 0, fmt.Errorf("anthropic: unknown token type %d", tokenType)
	}
}

func (a *Anthropic) SupportsModel(model string) bool {
	_, ok := a.models[model]
	return ok
}
