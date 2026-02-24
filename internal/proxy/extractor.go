package proxy

import (
	"encoding/json"
	"strings"
)

// RequestInfo holds extracted information from an LLM API request.
type RequestInfo struct {
	Provider string
	Model    string
	Messages string // Concatenated message content for token counting
}

// ResponseUsage holds extracted token usage from an LLM API response.
type ResponseUsage struct {
	InputTokens  int64
	OutputTokens int64
	Model        string
}

// DetectProvider determines the provider from the request URL or path.
func DetectProvider(host, path string) string {
	host = strings.ToLower(host)
	path = strings.ToLower(path)

	switch {
	case strings.Contains(host, "openai.com") || strings.HasPrefix(path, "/v1/chat/completions"):
		return "openai"
	case strings.Contains(host, "anthropic.com") || strings.HasPrefix(path, "/v1/messages"):
		return "anthropic"
	default:
		return ""
	}
}

// ExtractRequestInfo extracts model and message content from the request body.
func ExtractRequestInfo(body []byte, provider string) (*RequestInfo, error) {
	switch provider {
	case "openai":
		return extractOpenAIRequest(body)
	case "anthropic":
		return extractAnthropicRequest(body)
	default:
		return nil, nil
	}
}

// ExtractResponseUsage extracts token usage from the API response body.
func ExtractResponseUsage(body []byte, provider string) (*ResponseUsage, error) {
	switch provider {
	case "openai":
		return extractOpenAIResponse(body)
	case "anthropic":
		return extractAnthropicResponse(body)
	default:
		return nil, nil
	}
}

func extractOpenAIRequest(body []byte) (*RequestInfo, error) {
	var req openAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	var content strings.Builder
	for _, msg := range req.Messages {
		content.WriteString(msg.Content)
		content.WriteString("\n")
	}

	return &RequestInfo{
		Provider: "openai",
		Model:    req.Model,
		Messages: content.String(),
	}, nil
}

func extractAnthropicRequest(body []byte) (*RequestInfo, error) {
	var req anthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	var content strings.Builder
	if req.System != "" {
		content.WriteString(req.System)
		content.WriteString("\n")
	}
	for _, msg := range req.Messages {
		content.WriteString(msg.Content)
		content.WriteString("\n")
	}

	return &RequestInfo{
		Provider: "anthropic",
		Model:    req.Model,
		Messages: content.String(),
	}, nil
}

func extractOpenAIResponse(body []byte) (*ResponseUsage, error) {
	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &ResponseUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		Model:        resp.Model,
	}, nil
}

func extractAnthropicResponse(body []byte) (*ResponseUsage, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &ResponseUsage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		Model:        resp.Model,
	}, nil
}

// OpenAI request/response structures

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Model string       `json:"model"`
	Usage openAIUsage  `json:"usage"`
}

type openAIUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// Anthropic request/response structures

type anthropicRequest struct {
	Model    string             `json:"model"`
	System   string             `json:"system,omitempty"`
	Messages []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Model string          `json:"model"`
	Usage anthropicUsage  `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}
