package tracker_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/alerts"
	"github.com/yapay-ai/llm-cost-guardian/pkg/model"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
)

func newTestBudgetManager(t *testing.T, notifiers []alerts.Notifier) (*tracker.BudgetManager, storage.Storage) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := tracker.NewBudgetManager(store, notifiers, logger)
	return mgr, store
}

func TestBudgetManager_RecordSpend(t *testing.T) {
	mgr, store := newTestBudgetManager(t, nil)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "test",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}
	require.NoError(t, store.SetBudget(ctx, budget))

	err := mgr.RecordSpend(ctx, 25.00)
	require.NoError(t, err)

	got, err := store.GetBudget(ctx, "test")
	require.NoError(t, err)
	assert.InDelta(t, 25.00, got.CurrentSpend, 0.001)
}

func TestBudgetManager_CheckAll_NoBudgets(t *testing.T) {
	mgr, _ := newTestBudgetManager(t, nil)
	err := mgr.CheckAll(context.Background())
	require.NoError(t, err)
}

func TestBudgetManager_CheckAll_Exceeded(t *testing.T) {
	mgr, store := newTestBudgetManager(t, nil)
	ctx := context.Background()

	budget := &model.Budget{
		Name:     "test",
		LimitUSD: 50.00,
		Period:   model.PeriodMonthly,
	}
	require.NoError(t, store.SetBudget(ctx, budget))
	require.NoError(t, store.UpdateBudgetSpend(ctx, "test", 60.00))

	err := mgr.CheckAll(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded")
}

func TestBudgetManager_AlertsTriggered(t *testing.T) {
	alertSent := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alertSent = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiers := []alerts.Notifier{
		alerts.NewWebhookNotifier(server.URL, ""),
	}
	mgr, store := newTestBudgetManager(t, notifiers)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "alert-test",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}
	require.NoError(t, store.SetBudget(ctx, budget))

	// Spend 85% - should trigger warning
	err := mgr.RecordSpend(ctx, 85.00)
	require.NoError(t, err)
	assert.True(t, alertSent)
}

func TestBudgetManager_NoAlert_UnderThreshold(t *testing.T) {
	alertSent := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alertSent = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiers := []alerts.Notifier{
		alerts.NewWebhookNotifier(server.URL, ""),
	}
	mgr, store := newTestBudgetManager(t, notifiers)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "under-test",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}
	require.NoError(t, store.SetBudget(ctx, budget))

	// Spend 50% - should NOT trigger
	err := mgr.RecordSpend(ctx, 50.00)
	require.NoError(t, err)
	assert.False(t, alertSent)
}

func TestBudgetManager_ResetBudgetSpend(t *testing.T) {
	mgr, store := newTestBudgetManager(t, nil)
	ctx := context.Background()

	budget := &model.Budget{
		Name:     "reset-test",
		LimitUSD: 100.00,
		Period:   model.PeriodMonthly,
	}
	require.NoError(t, store.SetBudget(ctx, budget))
	require.NoError(t, store.UpdateBudgetSpend(ctx, "reset-test", 75.00))

	err := mgr.ResetBudgetSpend(ctx, "reset-test")
	require.NoError(t, err)

	got, err := store.GetBudget(ctx, "reset-test")
	require.NoError(t, err)
	assert.InDelta(t, 0.0, got.CurrentSpend, 0.001)
}

func TestBudgetManager_CriticalAlert(t *testing.T) {
	alertSent := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alertSent = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiers := []alerts.Notifier{
		alerts.NewWebhookNotifier(server.URL, ""),
	}
	mgr, store := newTestBudgetManager(t, notifiers)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "critical-test",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}
	require.NoError(t, store.SetBudget(ctx, budget))

	// Spend 96% - should trigger critical
	err := mgr.RecordSpend(ctx, 96.00)
	require.NoError(t, err)
	assert.True(t, alertSent)
}

func TestBudgetManager_ExceededAlert(t *testing.T) {
	alertSent := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alertSent = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiers := []alerts.Notifier{
		alerts.NewWebhookNotifier(server.URL, ""),
	}
	mgr, store := newTestBudgetManager(t, notifiers)
	ctx := context.Background()

	budget := &model.Budget{
		Name:              "exceeded-test",
		LimitUSD:          100.00,
		Period:            model.PeriodMonthly,
		AlertThresholdPct: 80.0,
	}
	require.NoError(t, store.SetBudget(ctx, budget))

	// Spend 101% - should trigger exceeded
	err := mgr.RecordSpend(ctx, 101.00)
	require.NoError(t, err)
	assert.True(t, alertSent)
}
