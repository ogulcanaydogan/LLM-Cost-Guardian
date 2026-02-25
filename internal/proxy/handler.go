package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

// Handler is a transparent proxy that tracks LLM API costs.
type Handler struct {
	tracker        *tracker.UsageTracker
	defaultProject string
	addHeaders     bool
	denyOnExceed   bool
	logger         *slog.Logger
}

// NewHandler creates a new proxy handler.
func NewHandler(t *tracker.UsageTracker, defaultProject string, addHeaders, denyOnExceed bool, logger *slog.Logger) *Handler {
	return &Handler{
		tracker:        t,
		defaultProject: defaultProject,
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

	// Read request body for analysis
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(reqBody))

	// Detect provider and extract request info
	provider := DetectProvider(target.Host, r.URL.Path)
	if provider == "" {
		provider = r.Header.Get("X-LCG-Provider")
	}

	reqInfo, _ := ExtractRequestInfo(reqBody, provider)
	project := r.Header.Get("X-LCG-Project")
	if project == "" {
		project = h.defaultProject
	}

	// Budget pre-check
	if h.denyOnExceed {
		if checkErr := h.tracker.CheckBudget(r.Context()); checkErr != nil {
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
		},
		ModifyResponse: func(resp *http.Response) error {
			return h.captureResponse(resp, provider, reqInfo, project, start)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			h.logger.Error("proxy error", "error", err, "target", targetURL)
			http.Error(w, "proxy error: "+err.Error(), http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// captureResponse reads the upstream response, extracts usage, calculates cost, and injects headers.
func (h *Handler) captureResponse(resp *http.Response, provider string, reqInfo *RequestInfo, project string, start time.Time) error {
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

	model := usage.Model
	if model == "" && reqInfo != nil {
		model = reqInfo.Model
	}

	// Record usage
	record := &tracker.UsageRecord{
		ID:           uuid.New().String(),
		Provider:     provider,
		Model:        model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Project:      project,
		Timestamp:    time.Now().UTC(),
	}

	if trackErr := h.tracker.TrackWithTokens(context.Background(), record); trackErr != nil {
		h.logger.Error("failed to record usage", "error", trackErr)
	}

	// Add cost headers
	if h.addHeaders {
		latency := time.Since(start)
		resp.Header.Set("X-LLM-Cost", fmt.Sprintf("%.6f", record.CostUSD))
		resp.Header.Set("X-LLM-Input-Tokens", strconv.FormatInt(usage.InputTokens, 10))
		resp.Header.Set("X-LLM-Output-Tokens", strconv.FormatInt(usage.OutputTokens, 10))
		resp.Header.Set("X-LLM-Provider", provider)
		resp.Header.Set("X-LLM-Model", model)
		resp.Header.Set("X-LCG-Latency", latency.String())
	}

	// Restore body
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))

	return nil
}
