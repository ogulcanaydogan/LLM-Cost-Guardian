package providers

import "fmt"

// StaticProvider implements Provider for providers backed by static YAML pricing.
type StaticProvider struct {
	name   string
	config *ProviderConfig
	models map[string]ModelPricing
}

// NewStaticProvider creates a provider from static model pricing.
func NewStaticProvider(name string, cfg *ProviderConfig) *StaticProvider {
	models := make(map[string]ModelPricing, len(cfg.Models))
	for _, model := range cfg.Models {
		models[model.Model] = model
	}

	return &StaticProvider{
		name:   name,
		config: cfg,
		models: models,
	}
}

func (p *StaticProvider) Name() string {
	return p.name
}

func (p *StaticProvider) Models() []ModelPricing {
	return p.config.Models
}

func (p *StaticProvider) PricePerToken(model string, tokenType TokenType) (float64, error) {
	pricing, ok := p.models[model]
	if !ok {
		return 0, fmt.Errorf("%s: unknown model %q", p.name, model)
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
		return 0, fmt.Errorf("%s: unknown token type %d", p.name, tokenType)
	}
}

func (p *StaticProvider) SupportsModel(model string) bool {
	_, ok := p.models[model]
	return ok
}

// NewProvider builds a typed provider from pricing config.
func NewProvider(cfg *ProviderConfig) (Provider, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAI(cfg), nil
	case "anthropic":
		return NewAnthropic(cfg), nil
	case "azure-openai":
		return NewAzureOpenAI(cfg), nil
	case "bedrock":
		return NewBedrock(cfg), nil
	case "vertex-ai":
		return NewVertexAI(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// NewProviderFromFile loads pricing config and builds the matching provider.
func NewProviderFromFile(path string) (Provider, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewProvider(cfg)
}
