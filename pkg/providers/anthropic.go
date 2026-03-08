package providers

// Anthropic implements the Provider interface for Anthropic models.
type Anthropic struct {
	*StaticProvider
}

// NewAnthropic creates a new Anthropic provider from a pricing config.
func NewAnthropic(cfg *ProviderConfig) *Anthropic {
	return &Anthropic{StaticProvider: NewStaticProvider("anthropic", cfg)}
}

// NewAnthropicFromFile creates a new Anthropic provider from a YAML pricing file.
func NewAnthropicFromFile(path string) (*Anthropic, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewAnthropic(cfg), nil
}
