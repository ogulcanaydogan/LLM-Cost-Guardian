package providers

// Bedrock implements the Provider interface for AWS Bedrock models.
type Bedrock struct {
	*StaticProvider
}

// NewBedrock creates a new Bedrock provider from a pricing config.
func NewBedrock(cfg *ProviderConfig) *Bedrock {
	return &Bedrock{StaticProvider: NewStaticProvider("bedrock", cfg)}
}

// NewBedrockFromFile creates a new Bedrock provider from a YAML pricing file.
func NewBedrockFromFile(path string) (*Bedrock, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewBedrock(cfg), nil
}
