package server_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/server"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
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
	_, err = ut.Track(t.Context(), "openai", "gpt-4o", 1000, 500, "test")
	require.NoError(t, err)

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
}

func TestServer_Usage_WithFilters(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/usage?provider=openai&model=gpt-4o", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
