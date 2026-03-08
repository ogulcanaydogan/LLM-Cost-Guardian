package providers

// OpenAI implements the Provider interface for OpenAI models.
type OpenAI struct {
	*StaticProvider
}

// NewOpenAI creates a new OpenAI provider from a pricing config.
func NewOpenAI(cfg *ProviderConfig) *OpenAI {
	return &OpenAI{StaticProvider: NewStaticProvider("openai", cfg)}
}

// NewOpenAIFromFile creates a new OpenAI provider from a YAML pricing file.
func NewOpenAIFromFile(path string) (*OpenAI, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewOpenAI(cfg), nil
}
