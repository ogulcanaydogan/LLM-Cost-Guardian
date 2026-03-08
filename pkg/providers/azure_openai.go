package providers

// AzureOpenAI implements the Provider interface for Azure OpenAI deployments.
type AzureOpenAI struct {
	*StaticProvider
}

// NewAzureOpenAI creates a new Azure OpenAI provider from a pricing config.
func NewAzureOpenAI(cfg *ProviderConfig) *AzureOpenAI {
	return &AzureOpenAI{StaticProvider: NewStaticProvider("azure-openai", cfg)}
}

// NewAzureOpenAIFromFile creates a new Azure OpenAI provider from a YAML pricing file.
func NewAzureOpenAIFromFile(path string) (*AzureOpenAI, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewAzureOpenAI(cfg), nil
}
