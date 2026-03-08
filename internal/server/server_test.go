package server_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/server"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupServer(t *testing.T) *server.Server {
	t.Helper()
	registry := providers.NewRegistry()
	openai := providers.NewOpenAI(&providers.ProviderConfig{
		Provider: "openai",
		Models:   []providers.ModelPricing{{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00}},
	})
	_ = registry.Register(openai)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ut := tracker.NewUsageTracker(registry, store, nil, logger)

	// Seed some data
	_, err = ut.Track(t.Context(), "default", "openai", "gpt-4o", 1000, 500, "test")
	require.NoError(t, err)

	return server.NewServer(ut, logger)
}

func setupAnalyticsServer(t *testing.T) *server.Server {
	t.Helper()

	registry := providers.NewRegistry()
	require.NoError(t, registry.Register(providers.NewOpenAI(&providers.ProviderConfig{
		Provider: "openai",
		Models: []providers.ModelPricing{
			{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00},
			{Model: "gpt-4o-mini", InputPerMillion: 0.15, OutputPerMillion: 0.60},
		},
	})))
	require.NoError(t, registry.Register(providers.NewAnthropic(&providers.ProviderConfig{
		Provider: "anthropic",
		Models: []providers.ModelPricing{
			{Model: "claude-3.5-sonnet", InputPerMillion: 3.00, OutputPerMillion: 15.00},
		},
	})))

	dbPath := filepath.Join(t.TempDir(), "analytics.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ut := tracker.NewUsageTracker(registry, store, nil, logger)

	base := time.Now().UTC().AddDate(0, 0, -10)
	for day := 0; day < 7; day++ {
		require.NoError(t, ut.TrackWithTokens(context.Background(), &tracker.UsageRecord{
			Tenant:       "default",
			Provider:     "openai",
			Model:        "gpt-4o",
			InputTokens:  800,
			OutputTokens: 200,
			Project:      "payments",
			Timestamp:    base.AddDate(0, 0, day),
		}))
	}
	require.NoError(t, ut.TrackWithTokens(context.Background(), &tracker.UsageRecord{
		Tenant:       "default",
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  1200,
		OutputTokens: 6000,
		Project:      "payments",
		Metadata:     `{"prompt_chars":9000,"prompt_tokens_estimate":1200,"input_output_ratio":8,"large_static_context":true}`,
		Timestamp:    base.AddDate(0, 0, 7),
	}))

	return server.NewServer(ut, logger)
}

func TestServer_Health(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
}

func TestServer_Usage(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/usage", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var records []model.UsageRecord
	err := json.NewDecoder(w.Body).Decode(&records)
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestServer_Summary(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/summary?period=daily", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var summary model.UsageSummary
	err := json.NewDecoder(w.Body).Decode(&summary)
	require.NoError(t, err)
	assert.Equal(t, int64(1), summary.RecordCount)
	assert.Equal(t, summary.TotalCostUSD, summary.ByProject["test"])
}

func TestServer_Usage_WithFilters(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/usage?provider=openai&model=gpt-4o", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_Metrics(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "lcg_usage_records_total 1")
	assert.Contains(t, w.Body.String(), `lcg_requests_total{tenant="default",provider="openai",model="gpt-4o",project="test"} 1`)
}

func TestServer_IntelligenceEndpoints(t *testing.T) {
	srv := setupAnalyticsServer(t)

	tests := []struct {
		path string
	}{
		{path: "/api/v1/anomalies?tenant=default"},
		{path: "/api/v1/forecast?tenant=default"},
		{path: "/api/v1/recommendations?tenant=default"},
		{path: "/api/v1/prompt-optimizations?tenant=default"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, tt.path)
		assert.NotEqual(t, "[]\n", w.Body.String(), tt.path)
	}
}
