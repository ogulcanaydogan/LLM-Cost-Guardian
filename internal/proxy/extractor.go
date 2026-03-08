package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// RequestInfo holds extracted information from an LLM API request.
type RequestInfo struct {
	Provider     string
	Model        string
	Messages     string // Concatenated message content for token counting
	MessageCount int
	SystemChars  int
}

// ResponseUsage holds extracted token usage from an LLM API response.
type ResponseUsage struct {
	InputTokens  int64
	OutputTokens int64
	Model        string
}

// DetectProvider determines the provider from the request URL or path.
func DetectProvider(host, requestPath string) string {
	host = strings.ToLower(host)
	requestPath = strings.ToLower(requestPath)

	switch {
	case strings.Contains(host, ".openai.azure.com") || strings.Contains(host, ".services.ai.azure.com") || strings.Contains(requestPath, "/openai/deployments/"):
		return "azure-openai"
	case strings.Contains(host, "openai.com") || strings.HasPrefix(requestPath, "/v1/chat/completions"):
		return "openai"
	case strings.Contains(host, "anthropic.com") || strings.HasPrefix(requestPath, "/v1/messages"):
		return "anthropic"
	case strings.Contains(host, "bedrock-runtime.") || strings.Contains(host, "bedrock.") ||
		(strings.Contains(requestPath, "/model/") && (strings.Contains(requestPath, "/converse") || strings.Contains(requestPath, "/invoke"))):
		return "bedrock"
	case strings.Contains(host, "aiplatform.googleapis.com") || strings.Contains(host, "generativelanguage.googleapis.com") ||
		strings.Contains(requestPath, "/publishers/google/models/"):
		return "vertex-ai"
	default:
		return ""
	}
}

// ExtractRequestInfo extracts model and message content from the request body.
func ExtractRequestInfo(body []byte, provider string, endpointPath ...string) (*RequestInfo, error) {
	requestPath := firstPath(endpointPath)

	switch provider {
	case "openai":
		return extractOpenAIRequest(body)
	case "azure-openai":
		info, err := extractOpenAIRequest(body)
		if err != nil || info == nil {
			return info, err
		}
		info.Provider = "azure-openai"
		return info, nil
	case "anthropic":
		return extractAnthropicRequest(body)
	case "bedrock":
		return extractBedrockRequest(body, requestPath)
	case "vertex-ai":
		return extractVertexAIRequest(body, requestPath)
	default:
		return nil, nil
	}
}

// ExtractResponseUsage extracts token usage from the API response body.
func ExtractResponseUsage(body []byte, provider string) (*ResponseUsage, error) {
	switch provider {
	case "openai":
		return extractOpenAIResponse(body)
	case "azure-openai":
		usage, err := extractOpenAIResponse(body)
		if err != nil || usage == nil {
			return usage, err
		}
		return usage, nil
	case "anthropic":
		return extractAnthropicResponse(body)
	case "bedrock":
		return extractBedrockResponse(body)
	case "vertex-ai":
		return extractVertexAIResponse(body)
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
		Provider:     "openai",
		Model:        req.Model,
		Messages:     content.String(),
		MessageCount: len(req.Messages),
		SystemChars:  countOpenAISystemChars(req.Messages),
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
		Provider:     "anthropic",
		Model:        req.Model,
		Messages:     content.String(),
		MessageCount: len(req.Messages),
		SystemChars:  len(req.System),
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

func extractBedrockRequest(body []byte, endpointPath string) (*RequestInfo, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	var content strings.Builder
	appendTextContent(&content, req["system"])
	appendTextContent(&content, req["messages"])
	appendTextContent(&content, req["inputText"])
	appendTextContent(&content, req["prompt"])

	return &RequestInfo{
		Provider:     "bedrock",
		Model:        extractModelFromPath("bedrock", endpointPath),
		Messages:     strings.TrimSpace(content.String()),
		MessageCount: countMessages(req["messages"]),
	}, nil
}

func extractVertexAIRequest(body []byte, endpointPath string) (*RequestInfo, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	var content strings.Builder
	appendTextContent(&content, req["systemInstruction"])
	appendTextContent(&content, req["contents"])

	return &RequestInfo{
		Provider:     "vertex-ai",
		Model:        extractModelFromPath("vertex-ai", endpointPath),
		Messages:     strings.TrimSpace(content.String()),
		MessageCount: countMessages(req["contents"]),
		SystemChars:  countMessageChars(req["systemInstruction"]),
	}, nil
}

func extractBedrockResponse(body []byte) (*ResponseUsage, error) {
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	usageMap, ok := resp["usage"].(map[string]any)
	if !ok {
		return nil, nil
	}

	return &ResponseUsage{
		InputTokens:  int64Value(usageMap["inputTokens"]),
		OutputTokens: int64Value(usageMap["outputTokens"]),
		Model:        stringValue(resp["modelId"]),
	}, nil
}

func extractVertexAIResponse(body []byte) (*ResponseUsage, error) {
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	usageMap, ok := resp["usageMetadata"].(map[string]any)
	if !ok {
		return nil, nil
	}

	return &ResponseUsage{
		InputTokens:  int64Value(usageMap["promptTokenCount"]),
		OutputTokens: int64Value(usageMap["candidatesTokenCount"]),
		Model:        stringValue(resp["modelVersion"]),
	}, nil
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

func extractModelFromPath(provider, endpointPath string) string {
	if endpointPath == "" {
		return ""
	}

	switch provider {
	case "bedrock":
		const marker = "/model/"
		start := strings.Index(endpointPath, marker)
		if start == -1 {
			return ""
		}
		rest := endpointPath[start+len(marker):]
		end := strings.Index(rest, "/")
		if end == -1 {
			return decodePathValue(rest)
		}
		return decodePathValue(rest[:end])
	case "vertex-ai":
		re := regexp.MustCompile(`/publishers/[^/]+/models/([^:]+)(?::|$)`)
		matches := re.FindStringSubmatch(endpointPath)
		if len(matches) == 2 {
			return matches[1]
		}
		return path.Base(endpointPath)
	default:
		return ""
	}
}

func decodePathValue(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func appendTextContent(builder *strings.Builder, value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		if strings.TrimSpace(v) != "" {
			builder.WriteString(strings.TrimSpace(v))
			builder.WriteString("\n")
		}
	case []any:
		for _, item := range v {
			appendTextContent(builder, item)
		}
	case map[string]any:
		if text, ok := v["text"]; ok {
			appendTextContent(builder, text)
		}
		if content, ok := v["content"]; ok {
			appendTextContent(builder, content)
		}
		if parts, ok := v["parts"]; ok {
			appendTextContent(builder, parts)
		}
		if inlineData, ok := v["inlineData"]; ok {
			appendTextContent(builder, inlineData)
		}
	default:
		builder.WriteString(strings.TrimSpace(fmt.Sprint(v)))
		builder.WriteString("\n")
	}
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		i, _ := v.Int64()
		return i
	default:
		return 0
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func countOpenAISystemChars(messages []openAIMessage) int {
	total := 0
	for _, message := range messages {
		if strings.EqualFold(message.Role, "system") {
			total += len(message.Content)
		}
	}
	return total
}

func countMessages(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	default:
		return 0
	}
}

func countMessageChars(value any) int {
	switch v := value.(type) {
	case nil:
		return 0
	case string:
		return len(v)
	case []any:
		total := 0
		for _, item := range v {
			total += countMessageChars(item)
		}
		return total
	case map[string]any:
		total := 0
		for _, key := range []string{"text", "content", "parts"} {
			if child, ok := v[key]; ok {
				total += countMessageChars(child)
			}
		}
		return total
	default:
		return len(fmt.Sprint(v))
	}
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
	Model string      `json:"model"`
	Usage openAIUsage `json:"usage"`
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
	Model string         `json:"model"`
	Usage anthropicUsage `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}
