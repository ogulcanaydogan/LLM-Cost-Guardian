package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
)

// Server provides health check and metrics API endpoints.
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
	s.mux.HandleFunc("GET /api/v1/usage", s.handleUsage)
	s.mux.HandleFunc("GET /api/v1/summary", s.handleSummary)
}

// Handler returns the HTTP handler for this server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := tracker.ReportFilter{
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
	json.NewEncoder(w).Encode(records)
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
	json.NewEncoder(w).Encode(summary)
}
