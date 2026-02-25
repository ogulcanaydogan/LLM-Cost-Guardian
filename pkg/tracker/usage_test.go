package tracker_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

func newTestTracker(t *testing.T) (*tracker.UsageTracker, storage.Storage) {
	t.Helper()
	registry := newTestRegistry(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	budgetMgr := tracker.NewBudgetManager(store, nil, logger)
	ut := tracker.NewUsageTracker(registry, store, budgetMgr, logger)
	return ut, store
}

func TestUsageTracker_Track(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	record, err := ut.Track(ctx, "openai", "gpt-4o", 1000, 500, "test-project")
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, "openai", record.Provider)
	assert.Equal(t, "gpt-4o", record.Model)
	assert.Equal(t, int64(1000), record.InputTokens)
	assert.Equal(t, int64(500), record.OutputTokens)
	assert.Greater(t, record.CostUSD, 0.0)
	assert.Equal(t, "test-project", record.Project)
}

func TestUsageTracker_Track_UnknownProvider(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	_, err := ut.Track(ctx, "unknown", "model", 100, 50, "test")
	assert.Error(t, err)
}

func TestUsageTracker_TrackWithTokens(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	record := &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  500,
		OutputTokens: 200,
		Project:      "test",
	}

	err := ut.TrackWithTokens(ctx, record)
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.Greater(t, record.CostUSD, 0.0)
}

func TestUsageTracker_TrackWithTokens_PresetCost(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	record := &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  500,
		OutputTokens: 200,
		CostUSD:      0.123,
		Project:      "test",
	}

	err := ut.TrackWithTokens(ctx, record)
	require.NoError(t, err)
	assert.Equal(t, 0.123, record.CostUSD)
}

func TestUsageTracker_Report(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	_, err := ut.Track(ctx, "openai", "gpt-4o", 1000, 500, "test")
	require.NoError(t, err)
	_, err = ut.Track(ctx, "anthropic", "claude-3.5-sonnet", 2000, 1000, "test")
	require.NoError(t, err)

	summary, err := ut.Report(ctx, model.ReportFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.RecordCount)
	assert.Greater(t, summary.TotalCostUSD, 0.0)
}

func TestUsageTracker_Query(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	_, err := ut.Track(ctx, "openai", "gpt-4o", 1000, 500, "proj-a")
	require.NoError(t, err)
	_, err = ut.Track(ctx, "openai", "gpt-4o-mini", 500, 200, "proj-b")
	require.NoError(t, err)

	records, err := ut.Query(ctx, model.ReportFilter{Project: "proj-a"})
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "proj-a", records[0].Project)
}

func TestUsageTracker_CheckBudget(t *testing.T) {
	ut, _ := newTestTracker(t)
	ctx := context.Background()

	// No budgets, should pass
	err := ut.CheckBudget(ctx)
	require.NoError(t, err)
}
