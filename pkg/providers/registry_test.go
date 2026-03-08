package providers_test

import (
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := providers.NewRegistry()
	p := newTestOpenAI(t)

	err := r.Register(p)
	require.NoError(t, err)

	got, err := r.Get("openai")
	require.NoError(t, err)
	assert.Equal(t, "openai", got.Name())
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := providers.NewRegistry()
	p := newTestOpenAI(t)

	err := r.Register(p)
	require.NoError(t, err)

	err = r.Register(p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := providers.NewRegistry()
	_, err := r.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_List(t *testing.T) {
	r := providers.NewRegistry()
	_ = r.Register(newTestOpenAI(t))
	_ = r.Register(newTestAnthropic(t))

	names := r.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "openai")
	assert.Contains(t, names, "anthropic")
}

func TestRegistry_All(t *testing.T) {
	r := providers.NewRegistry()
	_ = r.Register(newTestOpenAI(t))
	_ = r.Register(newTestAnthropic(t))

	all := r.All()
	assert.Len(t, all, 2)
}

func TestRegistry_FindProviderForModel(t *testing.T) {
	r := providers.NewRegistry()
	_ = r.Register(newTestOpenAI(t))
	_ = r.Register(newTestAnthropic(t))

	p, err := r.FindProviderForModel("gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())

	p, err = r.FindProviderForModel("claude-3.5-sonnet")
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name())

	_, err = r.FindProviderForModel("unknown-model")
	assert.Error(t, err)
}

func TestNewProvider_SupportedProviders(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
	}{
		{name: "openai", provider: "openai", model: "gpt-4o"},
		{name: "anthropic", provider: "anthropic", model: "claude-3.5-sonnet"},
		{name: "azure-openai", provider: "azure-openai", model: "gpt-4o"},
		{name: "bedrock", provider: "bedrock", model: "anthropic.claude-3-5-sonnet-20241022-v2:0"},
		{name: "vertex-ai", provider: "vertex-ai", model: "gemini-1.5-pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := providers.NewProvider(&providers.ProviderConfig{
				Provider: tt.provider,
				Models: []providers.ModelPricing{
					{Model: tt.model, InputPerMillion: 1.00, OutputPerMillion: 2.00},
				},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.provider, provider.Name())
			assert.True(t, provider.SupportsModel(tt.model))
		})
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	_, err := providers.NewProvider(&providers.ProviderConfig{
		Provider: "unknown",
		Models:   []providers.ModelPricing{{Model: "x", InputPerMillion: 1, OutputPerMillion: 1}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}
