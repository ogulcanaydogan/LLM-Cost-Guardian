package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/model"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
)

func newTestDB(t *testing.T) *storage.SQLite {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSQLite_RecordUsage(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	record := &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.0075,
		Project:      "test-project",
	}

	err := db.RecordUsage(ctx, record)
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.False(t, record.Timestamp.IsZero())
}

func TestSQLite_QueryUsage(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	records := []*model.UsageRecord{
		{Provider: "openai", Model: "gpt-4o", InputTokens: 100, OutputTokens: 50, CostUSD: 0.001, Project: "proj-a"},
		{Provider: "openai", Model: "gpt-4o-mini", InputTokens: 200, OutputTokens: 100, CostUSD: 0.0001, Project: "proj-a"},
		{Provider: "anthropic", Model: "claude-3.5-sonnet", InputTokens: 300, OutputTokens: 150, CostUSD: 0.003, Project: "proj-b"},
	}
	for _, r := range records {
		require.NoError(t, db.RecordUsage(ctx, r))
	}

	// Query all
	all, err := db.QueryUsage(ctx, model.ReportFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Filter by provider
	openai, err := db.QueryUsage(ctx, model.ReportFilter{Provider: "openai"})
	require.NoError(t, err)
	assert.Len(t, openai, 2)

	// Filter by project
	projB, err := db.QueryUsage(ctx, model.ReportFilter{Project: "proj-b"})
	require.NoError(t, err)
	assert.Len(t, projB, 1)

	// Filter by model
	mini, err := db.QueryUsage(ctx, model.ReportFilter{Model: "gpt-4o-mini"})
	require.NoError(t, err)
	assert.Len(t, mini, 1)
}

func TestSQLite_QueryUsage_TimeFilter(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	record := &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.001,
		Project:      "test",
		Timestamp:    now,
	}
	require.NoError(t, db.RecordUsage(ctx, record))

	// Should find within time window
	results, err := db.QueryUsage(ctx, model.ReportFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Should not find outside time window
	results, err = db.QueryUsage(ctx, model.ReportFilter{
		StartTime: now.Add(1 * time.Hour),
		EndTime:   now.Add(2 * time.Hour),
	})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestSQLite_AggregateUsage(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	records := []*model.UsageRecord{
		{Provider: "openai", Model: "gpt-4o", InputTokens: 100, OutputTokens: 50, CostUSD: 1.00, Project: "test"},
		{Provider: "openai", Model: "gpt-4o", InputTokens: 200, OutputTokens: 100, CostUSD: 2.00, Project: "test"},
		{Provider: "anthropic", Model: "claude-3.5-sonnet", InputTokens: 300, OutputTokens: 150, CostUSD: 3.00, Project: "test"},
	}
	for _, r := range records {
		require.NoError(t, db.RecordUsage(ctx, r))
	}

	summary, err := db.AggregateUsage(ctx, model.ReportFilter{})
	require.NoError(t, err)
	assert.InDelta(t, 6.00, summary.TotalCostUSD, 0.001)
	assert.Equal(t, int64(600), summary.TotalInputTokens)
	assert.Equal(t, int64(300), summary.TotalOutputTokens)
	assert.Equal(t, int64(3), summary.RecordCount)
	assert.InDelta(t, 3.00, summary.ByProvider["openai"], 0.001)
	assert.InDelta(t, 3.00, summary.ByProvider["anthropic"], 0.001)
	assert.InDelta(t, 3.00, summary.ByModel["gpt-4o"], 0.001)
}

func TestSQLite_Budget(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "test-budget",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}

	err := db.SetBudget(ctx, budget)
	require.NoError(t, err)

	got, err := db.GetBudget(ctx, "test-budget")
	require.NoError(t, err)
	assert.Equal(t, "test-budget", got.Name)
	assert.Equal(t, 100.00, got.LimitUSD)
	assert.Equal(t, model.PeriodMonthly, got.Period)
	assert.Equal(t, 80.0, got.AlertThresholdPct)
	assert.Equal(t, 0.0, got.CurrentSpend)
}

func TestSQLite_Budget_Update(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "update-test",
		LimitUSD:          50.00,
		Period:            model.PeriodDaily,
		AlertThresholdPct: 75.0,
	}
	require.NoError(t, db.SetBudget(ctx, budget))

	// Update with same name should upsert
	budget.LimitUSD = 100.00
	require.NoError(t, db.SetBudget(ctx, budget))

	got, err := db.GetBudget(ctx, "update-test")
	require.NoError(t, err)
	assert.Equal(t, 100.00, got.LimitUSD)
}

func TestSQLite_UpdateBudgetSpend(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	budget := &model.Budget{
		Name:     "spend-test",
		LimitUSD: 100.00,
		Period:   model.PeriodMonthly,
	}
	require.NoError(t, db.SetBudget(ctx, budget))

	require.NoError(t, db.UpdateBudgetSpend(ctx, "spend-test", 25.50))
	require.NoError(t, db.UpdateBudgetSpend(ctx, "spend-test", 10.25))

	got, err := db.GetBudget(ctx, "spend-test")
	require.NoError(t, err)
	assert.InDelta(t, 35.75, got.CurrentSpend, 0.001)
}

func TestSQLite_UpdateBudgetSpend_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.UpdateBudgetSpend(ctx, "nonexistent", 10.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSQLite_ListBudgets(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	budgets := []*model.Budget{
		{Name: "budget-a", LimitUSD: 50.00, Period: model.PeriodDaily},
		{Name: "budget-b", LimitUSD: 100.00, Period: model.PeriodMonthly},
	}
	for _, b := range budgets {
		require.NoError(t, db.SetBudget(ctx, b))
	}

	list, err := db.ListBudgets(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestSQLite_GetBudget_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.GetBudget(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSQLite_MigrationIdempotency(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open and close twice to verify migration idempotency
	db1, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	db1.Close()

	db2, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	db2.Close()
}
