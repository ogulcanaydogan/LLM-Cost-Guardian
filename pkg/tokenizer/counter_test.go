package tokenizer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tokenizer"
)

func TestCountTokens_OpenAI(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		model    string
		minCount int64
		maxCount int64
	}{
		{"short text gpt-4o", "Hello world", "gpt-4o", 1, 5},
		{"medium text gpt-4o", "The quick brown fox jumps over the lazy dog", "gpt-4o", 5, 15},
		{"empty text", "", "gpt-4o", 0, 0},
		{"gpt-4", "Hello world", "gpt-4", 1, 5},
		{"gpt-3.5-turbo", "Hello world", "gpt-3.5-turbo", 1, 5},
		{"unknown openai model falls back", "Hello world", "gpt-99", 1, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := tokenizer.CountTokens(tt.text, "openai", tt.model)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, tt.minCount)
			assert.LessOrEqual(t, count, tt.maxCount)
		})
	}
}

func TestCountTokens_Anthropic(t *testing.T) {
	text := "Hello, this is a test message for token counting."
	count, err := tokenizer.CountTokens(text, "anthropic", "claude-3.5-sonnet")
	require.NoError(t, err)

	// Character-based estimation: len/4
	expectedApprox := int64((len(text) + 3) / 4)
	assert.Equal(t, expectedApprox, count)
}

func TestCountTokens_EmptyText(t *testing.T) {
	count, err := tokenizer.CountTokens("", "openai", "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	count, err = tokenizer.CountTokens("   ", "anthropic", "claude-3.5-sonnet")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCountTokens_UnknownProvider(t *testing.T) {
	count, err := tokenizer.CountTokens("Hello world", "unknown", "model")
	require.NoError(t, err)
	assert.Greater(t, count, int64(0))
}

func TestCountChatTokens(t *testing.T) {
	messages := []map[string]string{
		{"role": "system", "content": "You are a helpful assistant."},
		{"role": "user", "content": "What is Go?"},
	}

	count, err := tokenizer.CountChatTokens(messages, "openai", "gpt-4o")
	require.NoError(t, err)
	assert.Greater(t, count, int64(10))
}

func BenchmarkCountTokens_OpenAI_Short(b *testing.B) {
	for b.Loop() {
		_, _ = tokenizer.CountTokens("Hello world", "openai", "gpt-4o")
	}
}

func BenchmarkCountTokens_OpenAI_Medium(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a benchmark test for token counting performance."
	for b.Loop() {
		_, _ = tokenizer.CountTokens(text, "openai", "gpt-4o")
	}
}

func BenchmarkCountTokens_Estimation(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a benchmark test for token counting performance."
	for b.Loop() {
		_, _ = tokenizer.CountTokens(text, "anthropic", "claude-3.5-sonnet")
	}
}
