package providers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
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
