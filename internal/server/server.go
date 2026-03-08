package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/httpauth"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

// Server provides health check and JSON usage API endpoints.
type Server struct {
	tracker *tracker.UsageTracker
	mux     *http.ServeMux
	logger  *slog.Logger
}

// NewServer creates an API server.
func NewServer(t *tracker.UsageTracker, logger *slog.Logger) *Server {
	s := &Server{
		tracker: t,
		mux:     http.NewServeMux(),
		logger:  logger,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /metrics", s.handleMetrics)
	s.mux.HandleFunc("GET /api/v1/usage", s.handleUsage)
	s.mux.HandleFunc("GET /api/v1/summary", s.handleSummary)
	s.mux.HandleFunc("GET /api/v1/anomalies", s.handleAnomalies)
	s.mux.HandleFunc("GET /api/v1/forecast", s.handleForecast)
	s.mux.HandleFunc("GET /api/v1/recommendations", s.handleRecommendations)
	s.mux.HandleFunc("GET /api/v1/prompt-optimizations", s.handlePromptOptimizations)
}

// Handler returns the HTTP handler for this server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		s.logger.Error("encode health response", "error", err)
	}
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := tracker.ReportFilter{
		Tenant:   tenantFilterFromRequest(r),
		Provider: r.URL.Query().Get("provider"),
		Model:    r.URL.Query().Get("model"),
		Project:  r.URL.Query().Get("project"),
	}

	records, err := s.tracker.Query(ctx, filter)
	if err != nil {
		s.logger.Error("query usage", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(records); err != nil {
		s.logger.Error("encode usage response", "error", err)
	}
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	period := tracker.BudgetPeriod(r.URL.Query().Get("period"))
	if period == "" {
		period = tracker.PeriodDaily
	}

	start, end := tracker.PeriodBounds(period)
	filter := tracker.ReportFilter{
		Tenant:    tenantFilterFromRequest(r),
		Provider:  r.URL.Query().Get("provider"),
		Project:   r.URL.Query().Get("project"),
		StartTime: start,
		EndTime:   end,
	}

	summary, err := s.tracker.Report(ctx, filter)
	if err != nil {
		s.logger.Error("aggregate usage", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		s.logger.Error("encode summary response", "error", err)
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := tracker.ReportFilter{Tenant: tenantFilterFromRequest(r)}

	summary, err := s.tracker.Report(ctx, filter)
	if err != nil {
		s.logger.Error("aggregate usage for metrics", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	records, err := s.tracker.Query(ctx, filter)
	if err != nil {
		s.logger.Error("query usage for metrics", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if _, err := fmt.Fprint(w, renderPrometheusMetrics(summary, records)); err != nil {
		s.logger.Error("write metrics response", "error", err)
	}
}

func tenantFilterFromRequest(r *http.Request) string {
	requested := r.URL.Query().Get("tenant")
	identity, ok := httpauth.IdentityFromContext(r.Context())
	if !ok {
		return requested
	}
	if identity.Admin && requested != "" {
		return requested
	}
	return identity.Tenant.Slug
}

func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := baseFilterFromRequest(r)
	anomalies, err := s.tracker.DetectAnomalies(ctx, filter)
	if err != nil {
		s.logger.Error("detect anomalies", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(anomalies); err != nil {
		s.logger.Error("encode anomalies response", "error", err)
	}
}

func (s *Server) handleForecast(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := baseFilterFromRequest(r)
	forecasts, err := s.tracker.Forecast(ctx, filter)
	if err != nil {
		s.logger.Error("forecast usage", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forecasts); err != nil {
		s.logger.Error("encode forecast response", "error", err)
	}
}

func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := baseFilterFromRequest(r)
	recommendations, err := s.tracker.RecommendModels(ctx, filter)
	if err != nil {
		s.logger.Error("recommend models", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(recommendations); err != nil {
		s.logger.Error("encode recommendations response", "error", err)
	}
}

func (s *Server) handlePromptOptimizations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := baseFilterFromRequest(r)
	optimizations, err := s.tracker.PromptOptimizations(ctx, filter)
	if err != nil {
		s.logger.Error("prompt optimizations", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(optimizations); err != nil {
		s.logger.Error("encode prompt optimizations response", "error", err)
	}
}

func baseFilterFromRequest(r *http.Request) tracker.ReportFilter {
	return tracker.ReportFilter{
		Tenant:   tenantFilterFromRequest(r),
		Provider: r.URL.Query().Get("provider"),
		Model:    r.URL.Query().Get("model"),
		Project:  r.URL.Query().Get("project"),
	}
}
