package providers

// VertexAI implements the Provider interface for Google Vertex AI models.
type VertexAI struct {
	*StaticProvider
}

// NewVertexAI creates a new Vertex AI provider from a pricing config.
func NewVertexAI(cfg *ProviderConfig) *VertexAI {
	return &VertexAI{StaticProvider: NewStaticProvider("vertex-ai", cfg)}
}

// NewVertexAIFromFile creates a new Vertex AI provider from a YAML pricing file.
func NewVertexAIFromFile(path string) (*VertexAI, error) {
	cfg, err := LoadPricing(path)
	if err != nil {
		return nil, err
	}
	return NewVertexAI(cfg), nil
}
