package proxy_test

import (
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		path     string
		expected string
	}{
		{"openai host", "api.openai.com", "/v1/chat/completions", "openai"},
		{"azure openai host", "example.openai.azure.com", "/openai/deployments/gpt-4o/chat/completions", "azure-openai"},
		{"openai path only", "localhost", "/v1/chat/completions", "openai"},
		{"anthropic host", "api.anthropic.com", "/v1/messages", "anthropic"},
		{"anthropic path only", "localhost", "/v1/messages", "anthropic"},
		{"bedrock host", "bedrock-runtime.us-east-1.amazonaws.com", "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/converse", "bedrock"},
		{"vertex host", "us-central1-aiplatform.googleapis.com", "/v1/projects/test/locations/us-central1/publishers/google/models/gemini-1.5-pro:generateContent", "vertex-ai"},
		{"unknown", "example.com", "/api/chat", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proxy.DetectProvider(tt.host, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRequestInfo_AzureOpenAI(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "Hello from Azure"}
		]
	}`)

	info, err := proxy.ExtractRequestInfo(body, "azure-openai")
	require.NoError(t, err)
	assert.Equal(t, "azure-openai", info.Provider)
	assert.Equal(t, "gpt-4o", info.Model)
	assert.Contains(t, info.Messages, "Hello from Azure")
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

func TestExtractRequestInfo_Bedrock(t *testing.T) {
	body := []byte(`{
		"messages": [
			{
				"role": "user",
				"content": [
					{"text": "Tell me a joke"}
				]
			}
		]
	}`)

	info, err := proxy.ExtractRequestInfo(body, "bedrock", "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/converse")
	require.NoError(t, err)
	assert.Equal(t, "bedrock", info.Provider)
	assert.Equal(t, "anthropic.claude-3-5-sonnet-20241022-v2:0", info.Model)
	assert.Contains(t, info.Messages, "Tell me a joke")
}

func TestExtractRequestInfo_VertexAI(t *testing.T) {
	body := []byte(`{
		"systemInstruction": {
			"parts": [{"text": "You are concise."}]
		},
		"contents": [
			{
				"role": "user",
				"parts": [{"text": "Summarize the release notes"}]
			}
		]
	}`)

	info, err := proxy.ExtractRequestInfo(body, "vertex-ai", "/v1/projects/demo/locations/us-central1/publishers/google/models/gemini-1.5-pro:generateContent")
	require.NoError(t, err)
	assert.Equal(t, "vertex-ai", info.Provider)
	assert.Equal(t, "gemini-1.5-pro", info.Model)
	assert.Contains(t, info.Messages, "You are concise")
	assert.Contains(t, info.Messages, "Summarize the release notes")
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

func TestExtractResponseUsage_Bedrock(t *testing.T) {
	body := []byte(`{
		"output": {
			"message": {
				"role": "assistant",
				"content": [{"text": "Hello"}]
			}
		},
		"usage": {
			"inputTokens": 30,
			"outputTokens": 12,
			"totalTokens": 42
		},
		"modelId": "anthropic.claude-3-5-sonnet-20241022-v2:0"
	}`)

	usage, err := proxy.ExtractResponseUsage(body, "bedrock")
	require.NoError(t, err)
	assert.Equal(t, int64(30), usage.InputTokens)
	assert.Equal(t, int64(12), usage.OutputTokens)
	assert.Equal(t, "anthropic.claude-3-5-sonnet-20241022-v2:0", usage.Model)
}

func TestExtractResponseUsage_VertexAI(t *testing.T) {
	body := []byte(`{
		"modelVersion": "gemini-1.5-pro",
		"usageMetadata": {
			"promptTokenCount": 44,
			"candidatesTokenCount": 19,
			"totalTokenCount": 63
		}
	}`)

	usage, err := proxy.ExtractResponseUsage(body, "vertex-ai")
	require.NoError(t, err)
	assert.Equal(t, int64(44), usage.InputTokens)
	assert.Equal(t, int64(19), usage.OutputTokens)
	assert.Equal(t, "gemini-1.5-pro", usage.Model)
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

	_, err = proxy.ExtractResponseUsage([]byte(`{invalid`), "bedrock")
	assert.Error(t, err)

	_, err = proxy.ExtractResponseUsage([]byte(`{invalid`), "vertex-ai")
	assert.Error(t, err)
}
