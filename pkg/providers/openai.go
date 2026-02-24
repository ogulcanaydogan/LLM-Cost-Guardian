package providers

import "fmt"

// OpenAI implements the Provider interface for OpenAI models.
type OpenAI struct {
	config *ProviderConfig
	models map[string]ModelPricing
}

// NewOpenAI creates a new OpenAI provider from a pricing config.
func NewOpenAI(cfg *ProviderConfig) *OpenAI {
	m := make(map[string]ModelPricing, len(cfg.Models))
	for _, model := range cfg.Models {
		m[model.Model] = model
	}
	return &OpenAI{config: cfg, models: m}
}

// NewOpenAIFromFile creates a new OpenAI provider from a YAML pricing file.
func NewOpenAIFromFile(path string) (*OpenAI, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewOpenAI(cfg), nil
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) Models() []ModelPricing {
	return o.config.Models
}

func (o *OpenAI) PricePerToken(model string, tokenType TokenType) (float64, error) {
	pricing, ok := o.models[model]
	if !ok {
		return 0, fmt.Errorf("openai: unknown model %q", model)
	}

	switch tokenType {
	case TokenInput:
		return pricing.InputPerMillion / 1_000_000, nil
	case TokenOutput:
		return pricing.OutputPerMillion / 1_000_000, nil
	case TokenCachedInput:
		return pricing.InputPerMillion / 1_000_000, nil // OpenAI doesn't differentiate cached
	default:
		return 0, fmt.Errorf("openai: unknown token type %d", tokenType)
	}
}

func (o *OpenAI) SupportsModel(model string) bool {
	_, ok := o.models[model]
	return ok
}
