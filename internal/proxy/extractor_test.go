package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/internal/proxy"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		path     string
		expected string
	}{
		{"openai host", "api.openai.com", "/v1/chat/completions", "openai"},
		{"openai path only", "localhost", "/v1/chat/completions", "openai"},
		{"anthropic host", "api.anthropic.com", "/v1/messages", "anthropic"},
		{"anthropic path only", "localhost", "/v1/messages", "anthropic"},
		{"unknown", "example.com", "/api/chat", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proxy.DetectProvider(tt.host, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRequestInfo_OpenAI(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Hello"}
		]
	}`)

	info, err := proxy.ExtractRequestInfo(body, "openai")
	require.NoError(t, err)
	assert.Equal(t, "openai", info.Provider)
	assert.Equal(t, "gpt-4o", info.Model)
	assert.Contains(t, info.Messages, "helpful assistant")
	assert.Contains(t, info.Messages, "Hello")
}

func TestExtractRequestInfo_Anthropic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3.5-sonnet",
		"system": "You are a coding assistant.",
		"messages": [
			{"role": "user", "content": "Write a function"}
		]
	}`)

	info, err := proxy.ExtractRequestInfo(body, "anthropic")
	require.NoError(t, err)
	assert.Equal(t, "anthropic", info.Provider)
	assert.Equal(t, "claude-3.5-sonnet", info.Model)
	assert.Contains(t, info.Messages, "coding assistant")
	assert.Contains(t, info.Messages, "Write a function")
}

func TestExtractRequestInfo_Unknown(t *testing.T) {
	info, err := proxy.ExtractRequestInfo([]byte(`{}`), "unknown")
	require.NoError(t, err)
	assert.Nil(t, info)
}

func TestExtractResponseUsage_OpenAI(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-abc123",
		"model": "gpt-4o",
		"usage": {
			"prompt_tokens": 24,
			"completion_tokens": 8,
			"total_tokens": 32
		}
	}`)

	usage, err := proxy.ExtractResponseUsage(body, "openai")
	require.NoError(t, err)
	assert.Equal(t, int64(24), usage.InputTokens)
	assert.Equal(t, int64(8), usage.OutputTokens)
	assert.Equal(t, "gpt-4o", usage.Model)
}

func TestExtractResponseUsage_Anthropic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3.5-sonnet",
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50
		}
	}`)

	usage, err := proxy.ExtractResponseUsage(body, "anthropic")
	require.NoError(t, err)
	assert.Equal(t, int64(100), usage.InputTokens)
	assert.Equal(t, int64(50), usage.OutputTokens)
	assert.Equal(t, "claude-3.5-sonnet", usage.Model)
}

func TestExtractResponseUsage_Unknown(t *testing.T) {
	usage, err := proxy.ExtractResponseUsage([]byte(`{}`), "unknown")
	require.NoError(t, err)
	assert.Nil(t, usage)
}

func TestExtractRequestInfo_InvalidJSON(t *testing.T) {
	_, err := proxy.ExtractRequestInfo([]byte(`{invalid`), "openai")
	assert.Error(t, err)

	_, err = proxy.ExtractRequestInfo([]byte(`{invalid`), "anthropic")
	assert.Error(t, err)
}

func TestExtractResponseUsage_InvalidJSON(t *testing.T) {
	_, err := proxy.ExtractResponseUsage([]byte(`{invalid`), "openai")
	assert.Error(t, err)

	_, err = proxy.ExtractResponseUsage([]byte(`{invalid`), "anthropic")
	assert.Error(t, err)
}
