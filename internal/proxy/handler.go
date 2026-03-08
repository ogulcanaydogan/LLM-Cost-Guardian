package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/httpauth"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

// Handler is a transparent proxy that tracks LLM API costs.
type Handler struct {
	tracker        *tracker.UsageTracker
	defaultProject string
	maxBodySize    int64
	addHeaders     bool
	denyOnExceed   bool
	logger         *slog.Logger
}

// NewHandler creates a new proxy handler.
func NewHandler(t *tracker.UsageTracker, defaultProject string, maxBodySize int64, addHeaders, denyOnExceed bool, logger *slog.Logger) *Handler {
	return &Handler{
		tracker:        t,
		defaultProject: defaultProject,
		maxBodySize:    maxBodySize,
		addHeaders:     addHeaders,
		denyOnExceed:   denyOnExceed,
		logger:         logger,
	}
}

// ServeHTTP handles proxied requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Determine target from X-LCG-Target header or original Host
	targetURL := r.Header.Get("X-LCG-Target")
	if targetURL == "" {
		http.Error(w, "missing X-LCG-Target header", http.StatusBadRequest)
		return
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "invalid target URL", http.StatusBadRequest)
		return
	}

	if h.maxBodySize > 0 && r.ContentLength > h.maxBodySize {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	bodyReader := r.Body
	if h.maxBodySize > 0 {
		bodyReader = http.MaxBytesReader(w, r.Body, h.maxBodySize)
	}

	// Read request body for analysis
	reqBody, err := io.ReadAll(bodyReader)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(reqBody))

	// Detect provider and extract request info
	provider := strings.ToLower(strings.TrimSpace(r.Header.Get("X-LCG-Provider")))
	if provider == "" {
		provider = DetectProvider(target.Host, target.Path)
	}

	reqInfo, _ := ExtractRequestInfo(reqBody, provider, target.Path)
	streamingRequest := isStreamingRequest(r, target.Path, reqBody)
	project := r.Header.Get("X-LCG-Project")
	if project == "" {
		project = h.defaultProject
	}
	tenant := defaultTenant(r.Context())

	// Budget pre-check
	if h.denyOnExceed {
		if checkErr := h.tracker.CheckBudgetForProject(r.Context(), tenant, project); checkErr != nil {
			http.Error(w, fmt.Sprintf("budget exceeded: %v", checkErr), http.StatusPaymentRequired)
			return
		}
	}

	// Set up reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target
			req.Host = target.Host
			// Remove proxy-specific headers
			req.Header.Del("X-LCG-Target")
			req.Header.Del("X-LCG-Provider")
			req.Header.Del("X-LCG-Project")
			req.Header.Del("X-LCG-API-Key")
			req.Header.Del("X-LCG-Tenant")
		},
		ModifyResponse: func(resp *http.Response) error {
			return h.captureResponse(r.Context(), resp, provider, reqInfo, tenant, project, streamingRequest, start)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			h.logger.Error("proxy error", "error", err, "target", targetURL)
			http.Error(w, "proxy error: "+err.Error(), http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// captureResponse reads the upstream response, extracts usage, calculates cost, and injects headers.
func (h *Handler) captureResponse(ctx context.Context, resp *http.Response, provider string, reqInfo *RequestInfo, tenant, project string, streamingRequest bool, start time.Time) error {
	if streamingRequest || isStreamingContentType(resp.Header.Get("Content-Type")) {
		return h.captureStreamingResponse(ctx, resp, provider, reqInfo, tenant, project, start)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	resp.Body.Close()

	// Extract usage from response
	usage, err := ExtractResponseUsage(body, provider)
	if err != nil {
		h.logger.Warn("failed to extract usage from response", "error", err)
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}

	if usage == nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}

	modelName := usage.Model
	if modelName == "" && reqInfo != nil {
		modelName = reqInfo.Model
	}

	// Record usage
	record := &tracker.UsageRecord{
		ID:           uuid.New().String(),
		Tenant:       tenant,
		Provider:     provider,
		Model:        modelName,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Project:      project,
		Metadata:     usageMetadataJSON(reqInfo, usage, false),
		Timestamp:    time.Now().UTC(),
	}

	if trackErr := h.tracker.TrackWithTokens(ctx, record); trackErr != nil {
		h.logger.Error("failed to record usage", "error", trackErr)
	}

	// Add cost headers
	if h.addHeaders {
		latency := time.Since(start)
		resp.Header.Set("X-LLM-Cost", fmt.Sprintf("%.6f", record.CostUSD))
		resp.Header.Set("X-LLM-Input-Tokens", strconv.FormatInt(usage.InputTokens, 10))
		resp.Header.Set("X-LLM-Output-Tokens", strconv.FormatInt(usage.OutputTokens, 10))
		resp.Header.Set("X-LLM-Provider", provider)
		resp.Header.Set("X-LLM-Model", modelName)
		resp.Header.Set("X-LCG-Latency", latency.String())
	}

	// Restore body
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))

	return nil
}

func defaultTenant(ctx context.Context) string {
	if identity, ok := httpauth.IdentityFromContext(ctx); ok && identity.Tenant.Slug != "" {
		return identity.Tenant.Slug
	}
	return "default"
}

func usageMetadataJSON(reqInfo *RequestInfo, usage *ResponseUsage, streaming bool) string {
	if reqInfo == nil || usage == nil {
		return "{}"
	}

	metadata := model.UsageMetadata{
		PromptChars:            len(strings.TrimSpace(reqInfo.Messages)),
		PromptTokensEstimate:   usage.InputTokens,
		SystemPromptChars:      reqInfo.SystemChars,
		MessageCount:           reqInfo.MessageCount,
		RepeatedLineRatio:      repeatedLineRatio(reqInfo.Messages),
		LargeStaticContext:     len(reqInfo.Messages) >= 6000 || reqInfo.SystemChars >= 1200,
		CachedContextCandidate: reqInfo.SystemChars >= 600 || len(reqInfo.Messages) >= 4000,
		InputOutputRatio:       safeRatio(float64(usage.InputTokens), float64(usage.OutputTokens)),
		Streaming:              streaming,
	}

	payload, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func repeatedLineRatio(content string) float64 {
	lines := strings.FieldsFunc(content, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	if len(lines) == 0 {
		return 0
	}

	seen := make(map[string]int)
	repeated := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		seen[line]++
		if seen[line] > 1 {
			repeated++
		}
	}
	return float64(repeated) / float64(len(lines))
}

func safeRatio(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return numerator
	}
	return numerator / denominator
}
