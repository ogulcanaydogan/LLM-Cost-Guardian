package tracker_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntelligenceTracker(t *testing.T) (*tracker.UsageTracker, storage.Storage) {
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
			{Model: "claude-3.5-haiku", InputPerMillion: 0.80, OutputPerMillion: 4.00},
		},
	})))

	store, err := storage.NewSQLite(filepath.Join(t.TempDir(), "intelligence.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return tracker.NewUsageTracker(registry, store, nil, logger), store
}

func seedUsageRecord(t *testing.T, usageTracker *tracker.UsageTracker, tenant, provider, modelName, project string, inputTokens, outputTokens int64, timestamp time.Time, metadata model.UsageMetadata) {
	t.Helper()

	rawMetadata, err := json.Marshal(metadata)
	require.NoError(t, err)

	require.NoError(t, usageTracker.TrackWithTokens(context.Background(), &tracker.UsageRecord{
		ID:           uuid.New().String(),
		Tenant:       tenant,
		Provider:     provider,
		Model:        modelName,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Project:      project,
		Metadata:     string(rawMetadata),
		Timestamp:    timestamp,
	}))
}

func TestUsageTracker_DetectAnomalies(t *testing.T) {
	usageTracker, _ := setupIntelligenceTracker(t)
	base := time.Now().UTC().AddDate(0, 0, -10)

	for day := 0; day < 7; day++ {
		seedUsageRecord(t, usageTracker, "default", "openai", "gpt-4o", "payments", 1000, 250, base.AddDate(0, 0, day), model.UsageMetadata{})
	}
	seedUsageRecord(t, usageTracker, "default", "openai", "gpt-4o", "payments", 1000, 5000, base.AddDate(0, 0, 7), model.UsageMetadata{})

	anomalies, err := usageTracker.DetectAnomalies(context.Background(), tracker.ReportFilter{Tenant: "default"})
	require.NoError(t, err)
	require.NotEmpty(t, anomalies)
	assert.Equal(t, "default", anomalies[0].Tenant)
	assert.Equal(t, "payments", anomalies[0].Project)
	assert.Contains(t, []string{"warning", "critical"}, anomalies[0].Severity)
}

func TestUsageTracker_DetectAnomalies_IgnoresFlatSeries(t *testing.T) {
	usageTracker, _ := setupIntelligenceTracker(t)
	base := time.Now().UTC().AddDate(0, 0, -10)

	for day := 0; day < 8; day++ {
		seedUsageRecord(t, usageTracker, "default", "openai", "gpt-4o", "stable", 1000, 250, base.AddDate(0, 0, day), model.UsageMetadata{})
	}

	anomalies, err := usageTracker.DetectAnomalies(context.Background(), tracker.ReportFilter{Tenant: "default", Project: "stable"})
	require.NoError(t, err)
	assert.Empty(t, anomalies)
}

func TestUsageTracker_Forecast(t *testing.T) {
	usageTracker, _ := setupIntelligenceTracker(t)
	base := time.Now().UTC().AddDate(0, 0, -14)

	for day := 0; day < 12; day++ {
		seedUsageRecord(t, usageTracker, "default", "openai", "gpt-4o", "growth", 800+int64(day*25), 200+int64(day*20), base.AddDate(0, 0, day), model.UsageMetadata{})
	}

	forecasts, err := usageTracker.Forecast(context.Background(), tracker.ReportFilter{Tenant: "default", Project: "growth"})
	require.NoError(t, err)
	require.Len(t, forecasts, 2)
	assert.Equal(t, 7, forecasts[0].HorizonDays)
	assert.Equal(t, 30, forecasts[1].HorizonDays)
	assert.Greater(t, forecasts[0].ForecastCostUSD, 0.0)
}

func TestUsageTracker_RecommendModels(t *testing.T) {
	usageTracker, _ := setupIntelligenceTracker(t)
	base := time.Now().UTC().AddDate(0, 0, -5)

	for day := 0; day < 4; day++ {
		seedUsageRecord(t, usageTracker, "default", "openai", "gpt-4o", "assistant", 2000, 700, base.AddDate(0, 0, day), model.UsageMetadata{})
	}

	recommendations, err := usageTracker.RecommendModels(context.Background(), tracker.ReportFilter{Tenant: "default", Project: "assistant"})
	require.NoError(t, err)
	require.NotEmpty(t, recommendations)
	assert.Equal(t, "gpt-4o", recommendations[0].CurrentModel)
	assert.Equal(t, "gpt-4o-mini", recommendations[0].SuggestedModel)
	assert.Greater(t, recommendations[0].EstimatedSavingsUSD, 0.0)
}

func TestUsageTracker_PromptOptimizations(t *testing.T) {
	usageTracker, _ := setupIntelligenceTracker(t)
	base := time.Now().UTC().AddDate(0, 0, -3)

	for day := 0; day < 3; day++ {
		seedUsageRecord(t, usageTracker, "default", "anthropic", "claude-3.5-sonnet", "docs", 4200, 300, base.AddDate(0, 0, day), model.UsageMetadata{
			PromptChars:            9000,
			PromptTokensEstimate:   4200,
			SystemPromptChars:      1200,
			MessageCount:           6,
			RepeatedLineRatio:      0.35,
			LargeStaticContext:     true,
			CachedContextCandidate: true,
			InputOutputRatio:       14,
		})
	}

	optimizations, err := usageTracker.PromptOptimizations(context.Background(), tracker.ReportFilter{Tenant: "default", Project: "docs"})
	require.NoError(t, err)
	require.NotEmpty(t, optimizations)
	assert.Equal(t, "anthropic", optimizations[0].Provider)
	assert.NotEmpty(t, optimizations[0].Suggestion)
}
