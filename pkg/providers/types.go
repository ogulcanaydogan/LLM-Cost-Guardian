package providers

// TokenType distinguishes input from output tokens for pricing.
type TokenType int

const (
	TokenInput       TokenType = iota // Standard input tokens
	TokenOutput                       // Standard output tokens
	TokenCachedInput                  // Cached input tokens (Anthropic)
)

// ModelPricing contains per-model pricing information.
type ModelPricing struct {
	Model                 string  `yaml:"model"`
	InputPerMillion       float64 `yaml:"input_per_million"`
	OutputPerMillion      float64 `yaml:"output_per_million"`
	CachedInputPerMillion float64 `yaml:"cached_input_per_million,omitempty"`
}

// ProviderConfig holds YAML-loaded pricing data for a provider.
type ProviderConfig struct {
	Provider string         `yaml:"provider"`
	Updated  string         `yaml:"updated"`
	Models   []ModelPricing `yaml:"models"`
}

// Provider is the core interface for LLM cost providers.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string

	// Models returns all known models with pricing.
	Models() []ModelPricing

	// PricePerToken returns the cost for a single token of the given type and model.
	PricePerToken(model string, tokenType TokenType) (float64, error)

	// SupportsModel reports whether this provider has pricing for the given model.
	SupportsModel(model string) bool
}
