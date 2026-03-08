package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Project:           "proj-a",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}

	err := db.SetBudget(ctx, budget)
	require.NoError(t, err)

	got, err := db.GetBudget(ctx, "test-budget")
	require.NoError(t, err)
	assert.Equal(t, "test-budget", got.Name)
	assert.Equal(t, "proj-a", got.Project)
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
		Project:           "proj-a",
		LimitUSD:          50.00,
		Period:            model.PeriodDaily,
		AlertThresholdPct: 75.0,
	}
	require.NoError(t, db.SetBudget(ctx, budget))

	// Update with same name should upsert
	budget.LimitUSD = 100.00
	budget.Project = "proj-b"
	require.NoError(t, db.SetBudget(ctx, budget))

	got, err := db.GetBudget(ctx, "update-test")
	require.NoError(t, err)
	assert.Equal(t, 100.00, got.LimitUSD)
	assert.Equal(t, "proj-b", got.Project)
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
		{Name: "budget-a", Project: "proj-a", LimitUSD: 50.00, Period: model.PeriodDaily},
		{Name: "budget-b", LimitUSD: 100.00, Period: model.PeriodMonthly},
	}
	for _, b := range budgets {
		require.NoError(t, db.SetBudget(ctx, b))
	}

	list, err := db.ListBudgets(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "proj-a", list[0].Project)
	assert.Equal(t, "", list[1].Project)
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

func TestSQLite_TenantsAndAPIKeys(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	defaultTenant, err := db.GetTenant(ctx, "default")
	require.NoError(t, err)
	assert.Equal(t, "default", defaultTenant.Slug)

	tenant := &model.Tenant{Slug: "acme-labs"}
	require.NoError(t, db.CreateTenant(ctx, tenant))
	assert.Equal(t, "Acme Labs", tenant.Name)

	ensured, err := db.EnsureTenant(ctx, "acme-labs", "")
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, ensured.ID)

	tenants, err := db.ListTenants(ctx)
	require.NoError(t, err)
	require.Len(t, tenants, 2)

	key := &model.APIKey{
		Tenant:    "acme-labs",
		Name:      "primary",
		KeyPrefix: "lcg_prefix",
		KeyHash:   "hash123",
		Status:    model.APIKeyStatusActive,
	}
	require.NoError(t, db.CreateAPIKey(ctx, key))

	keys, err := db.ListAPIKeys(ctx, "acme-labs")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "primary", keys[0].Name)

	resolvedKey, resolvedTenant, err := db.ResolveAPIKey(ctx, "hash123")
	require.NoError(t, err)
	assert.Equal(t, key.ID, resolvedKey.ID)
	assert.Equal(t, "acme-labs", resolvedTenant.Slug)
	require.NotNil(t, resolvedKey.LastUsedAt)

	require.NoError(t, db.RevokeAPIKey(ctx, key.ID))
	keys, err = db.ListAPIKeys(ctx, "acme-labs")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, model.APIKeyStatusRevoked, keys[0].Status)

	require.NoError(t, db.DisableTenant(ctx, "acme-labs"))
	disabled, err := db.GetTenant(ctx, "acme-labs")
	require.NoError(t, err)
	assert.Equal(t, model.TenantStatusDisabled, disabled.Status)
}

func TestSQLite_QueryUsageRollups(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, time.January, 15, 9, 30, 0, 0, time.UTC)

	require.NoError(t, db.RecordUsage(ctx, &model.UsageRecord{
		Tenant:       "default",
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      1.00,
		Project:      "proj-a",
		Timestamp:    base,
	}))
	require.NoError(t, db.RecordUsage(ctx, &model.UsageRecord{
		Tenant:       "default",
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  200,
		OutputTokens: 75,
		CostUSD:      2.50,
		Project:      "proj-a",
		Timestamp:    base.Add(20 * time.Minute),
	}))
	require.NoError(t, db.RecordUsage(ctx, &model.UsageRecord{
		Tenant:       "default",
		Provider:     "anthropic",
		Model:        "claude-3.5-sonnet",
		InputTokens:  300,
		OutputTokens: 125,
		CostUSD:      3.75,
		Project:      "proj-b",
		Timestamp:    base.Add(24 * time.Hour),
	}))

	hourly, err := db.QueryUsageRollups(ctx, model.ReportFilter{Tenant: "default", Project: "proj-a"}, "hourly", base.Add(-time.Hour), base.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, hourly, 1)
	assert.Equal(t, int64(2), hourly[0].RequestCount)
	assert.Equal(t, int64(300), hourly[0].InputTokens)
	assert.Equal(t, int64(125), hourly[0].OutputTokens)
	assert.InDelta(t, 3.50, hourly[0].CostUSD, 0.001)

	daily, err := db.QueryUsageRollups(ctx, model.ReportFilter{Tenant: "default"}, "daily", base.Add(-24*time.Hour), base.Add(48*time.Hour))
	require.NoError(t, err)
	require.Len(t, daily, 2)
	assert.Equal(t, "proj-a", daily[0].Project)
	assert.Equal(t, "proj-b", daily[1].Project)
}
