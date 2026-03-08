package proxy

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

const maxBufferedStreamText = 1 << 20

type streamCaptureResult struct {
	usage   *ResponseUsage
	content string
	rawSize int
}

type streamingBody struct {
	inner      io.ReadCloser
	parser     *streamParser
	onComplete func(streamCaptureResult)
	once       sync.Once
}

func newStreamingBody(inner io.ReadCloser, parser *streamParser, onComplete func(streamCaptureResult)) io.ReadCloser {
	return &streamingBody{
		inner:      inner,
		parser:     parser,
		onComplete: onComplete,
	}
}

func (s *streamingBody) Read(p []byte) (int, error) {
	n, err := s.inner.Read(p)
	if n > 0 {
		s.parser.Append(p[:n])
	}
	if err == io.EOF {
		s.finish()
	}
	return n, err
}

func (s *streamingBody) Close() error {
	s.finish()
	return s.inner.Close()
}

func (s *streamingBody) finish() {
	s.once.Do(func() {
		s.onComplete(s.parser.Finalize())
	})
}

type streamParser struct {
	provider   string
	reqInfo    *RequestInfo
	pending    string
	eventLines []string
	usage      *ResponseUsage
	outputText strings.Builder
	rawText    strings.Builder
	rawSize    int
}

func newStreamParser(provider string, reqInfo *RequestInfo) *streamParser {
	return &streamParser{
		provider: provider,
		reqInfo:  reqInfo,
	}
}

func (p *streamParser) Append(chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	p.rawSize += len(chunk)
	p.appendRawText(chunk)

	p.pending += string(chunk)
	for {
		idx := strings.IndexByte(p.pending, '\n')
		if idx < 0 {
			break
		}
		line := p.pending[:idx]
		p.pending = p.pending[idx+1:]
		p.processLine(strings.TrimSuffix(line, "\r"))
	}
}

func (p *streamParser) Finalize() streamCaptureResult {
	if strings.TrimSpace(p.pending) != "" {
		p.processLine(strings.TrimSuffix(p.pending, "\r"))
	}
	p.pending = ""
	p.flushEvent()

	usage := p.usage
	if usage == nil {
		usage = &ResponseUsage{}
	}
	if usage.Model == "" && p.reqInfo != nil {
		usage.Model = p.reqInfo.Model
	}
	if usage.InputTokens == 0 && p.reqInfo != nil {
		usage.InputTokens = estimateTokens(p.reqInfo.Messages)
	}

	content := strings.TrimSpace(p.outputText.String())
	if content == "" {
		content = strings.TrimSpace(p.rawText.String())
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = estimateTokens(content)
		if usage.OutputTokens == 0 && p.rawSize > 0 {
			usage.OutputTokens = int64(math.Ceil(float64(p.rawSize) / 4.0))
		}
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		usage = nil
	}

	return streamCaptureResult{
		usage:   usage,
		content: content,
		rawSize: p.rawSize,
	}
}

func (p *streamParser) appendRawText(chunk []byte) {
	if p.rawText.Len() >= maxBufferedStreamText {
		return
	}

	text := sanitizeChunk(string(chunk))
	if text == "" {
		return
	}

	remaining := maxBufferedStreamText - p.rawText.Len()
	if len(text) > remaining {
		text = text[:remaining]
	}
	p.rawText.WriteString(text)
}

func (p *streamParser) processLine(line string) {
	switch {
	case strings.HasPrefix(line, "event:"):
		return
	case strings.HasPrefix(line, "data:"):
		p.eventLines = append(p.eventLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
	case strings.TrimSpace(line) == "":
		p.flushEvent()
	default:
		payload := strings.TrimSpace(line)
		if strings.HasPrefix(payload, "{") || strings.HasPrefix(payload, "[") {
			p.processPayload(payload)
		}
	}
}

func (p *streamParser) flushEvent() {
	if len(p.eventLines) == 0 {
		return
	}
	payload := strings.Join(p.eventLines, "\n")
	p.eventLines = nil
	p.processPayload(payload)
}

func (p *streamParser) processPayload(payload string) {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" {
		return
	}

	if usage := extractStreamUsage([]byte(payload), p.provider); usage != nil {
		p.usage = mergeUsage(p.usage, usage)
	}

	text := extractStreamText([]byte(payload), p.provider)
	if text != "" && p.outputText.Len() < maxBufferedStreamText {
		remaining := maxBufferedStreamText - p.outputText.Len()
		if len(text) > remaining {
			text = text[:remaining]
		}
		p.outputText.WriteString(text)
	}
}

func (h *Handler) captureStreamingResponse(ctx context.Context, resp *http.Response, provider string, reqInfo *RequestInfo, tenant, project string, start time.Time) error {
	parser := newStreamParser(provider, reqInfo)
	model := ""
	if reqInfo != nil {
		model = reqInfo.Model
	}

	if h.addHeaders {
		resp.Header.Set("X-LCG-Streaming", "true")
		resp.Header.Set("X-LLM-Provider", provider)
		if model != "" {
			resp.Header.Set("X-LLM-Model", model)
		}
	}

	resp.Body = newStreamingBody(resp.Body, parser, func(result streamCaptureResult) {
		h.recordStreamingUsage(ctx, provider, reqInfo, tenant, project, start, result)
	})
	return nil
}

func (h *Handler) recordStreamingUsage(ctx context.Context, provider string, reqInfo *RequestInfo, tenant, project string, start time.Time, result streamCaptureResult) {
	if result.usage == nil {
		return
	}

	model := result.usage.Model
	if model == "" && reqInfo != nil {
		model = reqInfo.Model
	}
	if model == "" {
		h.logger.Warn("skipping streaming usage record without model", "provider", provider)
		return
	}

	record := &tracker.UsageRecord{
		ID:           uuid.New().String(),
		Tenant:       tenant,
		Provider:     provider,
		Model:        model,
		InputTokens:  result.usage.InputTokens,
		OutputTokens: result.usage.OutputTokens,
		Project:      project,
		Metadata:     usageMetadataJSON(reqInfo, result.usage, true),
		Timestamp:    time.Now().UTC(),
	}

	if trackErr := h.tracker.TrackWithTokens(ctx, record); trackErr != nil {
		h.logger.Error("failed to record streaming usage", "error", trackErr, "provider", provider, "model", model)
		return
	}

	if h.addHeaders {
		h.logger.Debug("streaming usage recorded",
			"provider", provider,
			"model", model,
			"tenant", tenant,
			"project", project,
			"input_tokens", result.usage.InputTokens,
			"output_tokens", result.usage.OutputTokens,
			"latency", time.Since(start).String(),
		)
	}
}

func isStreamingRequest(r *http.Request, targetPath string, body []byte) bool {
	if isStreamingContentType(r.Header.Get("Accept")) {
		return true
	}

	bodyText := strings.ToLower(string(body))
	if strings.Contains(bodyText, `"stream":true`) || strings.Contains(bodyText, `"stream": true`) {
		return true
	}

	return strings.Contains(strings.ToLower(targetPath), "stream")
}

func isStreamingContentType(contentType string) bool {
	value := strings.ToLower(contentType)
	return strings.Contains(value, "text/event-stream") ||
		strings.Contains(value, "application/x-ndjson") ||
		strings.Contains(value, "application/vnd.amazon.eventstream")
}

func extractStreamUsage(payload []byte, provider string) *ResponseUsage {
	usage, err := ExtractResponseUsage(payload, provider)
	if err == nil && usage != nil && (usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.Model != "") {
		return usage
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil
	}
	return recursiveUsage(decoded)
}

func recursiveUsage(value any) *ResponseUsage {
	switch typed := value.(type) {
	case map[string]any:
		usage := &ResponseUsage{}
		if token, ok := firstInt(typed, "prompt_tokens", "input_tokens", "inputTokens", "promptTokenCount", "inputTokenCount"); ok {
			usage.InputTokens = token
		}
		if token, ok := firstInt(typed, "completion_tokens", "output_tokens", "outputTokens", "candidatesTokenCount", "outputTokenCount"); ok {
			usage.OutputTokens = token
		}
		if modelName, ok := firstString(typed, "model", "modelId", "modelVersion"); ok {
			usage.Model = modelName
		}
		for _, nested := range typed {
			usage = mergeUsage(usage, recursiveUsage(nested))
		}
		if usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.Model != "" {
			return usage
		}
	case []any:
		var usage *ResponseUsage
		for _, nested := range typed {
			usage = mergeUsage(usage, recursiveUsage(nested))
		}
		if usage != nil && (usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.Model != "") {
			return usage
		}
	}
	return nil
}

func extractStreamText(payload []byte, provider string) string {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ""
	}

	switch provider {
	case "openai", "azure-openai":
		if text := extractOpenAIStreamText(decoded); text != "" {
			return text
		}
	case "anthropic":
		if text := extractAnthropicStreamText(decoded); text != "" {
			return text
		}
	case "vertex-ai":
		if text := extractVertexStreamText(decoded); text != "" {
			return text
		}
	case "bedrock":
		if text := extractBedrockStreamText(decoded); text != "" {
			return text
		}
	}

	return recursiveText(decoded)
}

func extractOpenAIStreamText(decoded map[string]any) string {
	choices, _ := decoded["choices"].([]any)
	var builder strings.Builder
	for _, rawChoice := range choices {
		choice, ok := rawChoice.(map[string]any)
		if !ok {
			continue
		}
		appendContentText(&builder, choice["delta"])
		appendContentText(&builder, choice["message"])
	}
	return builder.String()
}

func extractAnthropicStreamText(decoded map[string]any) string {
	var builder strings.Builder
	appendContentText(&builder, decoded["delta"])
	appendContentText(&builder, decoded["content_block"])
	appendContentText(&builder, decoded["message"])
	return builder.String()
}

func extractVertexStreamText(decoded map[string]any) string {
	var builder strings.Builder
	appendContentText(&builder, decoded["candidates"])
	appendContentText(&builder, decoded["content"])
	return builder.String()
}

func extractBedrockStreamText(decoded map[string]any) string {
	var builder strings.Builder
	appendContentText(&builder, decoded["delta"])
	appendContentText(&builder, decoded["output"])
	appendContentText(&builder, decoded["chunk"])
	return builder.String()
}

func appendContentText(builder *strings.Builder, value any) {
	switch typed := value.(type) {
	case string:
		builder.WriteString(typed)
	case map[string]any:
		if text, ok := typed["text"].(string); ok {
			builder.WriteString(text)
		}
		if text, ok := typed["outputText"].(string); ok {
			builder.WriteString(text)
		}
		if content, ok := typed["content"]; ok {
			appendContentText(builder, content)
		}
		if parts, ok := typed["parts"]; ok {
			appendContentText(builder, parts)
		}
		if delta, ok := typed["delta"]; ok {
			appendContentText(builder, delta)
		}
	case []any:
		for _, item := range typed {
			appendContentText(builder, item)
		}
	}
}

func recursiveText(value any) string {
	var builder strings.Builder
	appendRecursiveText(&builder, value)
	return builder.String()
}

func appendRecursiveText(builder *strings.Builder, value any) {
	switch typed := value.(type) {
	case map[string]any:
		appendContentText(builder, typed)
		for _, nested := range typed {
			appendRecursiveText(builder, nested)
		}
	case []any:
		for _, item := range typed {
			appendRecursiveText(builder, item)
		}
	default:
		appendContentText(builder, typed)
	}
}

func mergeUsage(current, next *ResponseUsage) *ResponseUsage {
	if next == nil {
		return current
	}
	if current == nil {
		usageCopy := *next
		return &usageCopy
	}
	if next.InputTokens > current.InputTokens {
		current.InputTokens = next.InputTokens
	}
	if next.OutputTokens > current.OutputTokens {
		current.OutputTokens = next.OutputTokens
	}
	if current.Model == "" && next.Model != "" {
		current.Model = next.Model
	}
	return current
}

func estimateTokens(content string) int64 {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}
	runes := utf8.RuneCountInString(content)
	return int64(math.Ceil(float64(runes) / 4.0))
}

func sanitizeChunk(chunk string) string {
	var builder strings.Builder
	for _, r := range chunk {
		switch {
		case unicode.IsGraphic(r) || unicode.IsSpace(r):
			builder.WriteRune(r)
		case r == utf8.RuneError:
			continue
		}
	}
	return builder.String()
}

func firstInt(values map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed), true
		case int64:
			return typed, true
		case int:
			return int64(typed), true
		case json.Number:
			parsed, err := typed.Int64()
			if err == nil {
				return parsed, true
			}
		case string:
			parsed, err := strconv.ParseInt(typed, 10, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func firstString(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if ok && strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	return "", false
}
