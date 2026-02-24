package tracker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
)

// UsageTracker is the main entry point for recording and querying LLM usage.
type UsageTracker struct {
	registry   *providers.Registry
	storage    storage.Storage
	calculator *CostCalculator
	budget     *BudgetManager
	logger     *slog.Logger
}

// NewUsageTracker creates a usage tracker with the given dependencies.
func NewUsageTracker(registry *providers.Registry, store storage.Storage, budget *BudgetManager, logger *slog.Logger) *UsageTracker {
	return &UsageTracker{
		registry:   registry,
		storage:    store,
		calculator: NewCostCalculator(registry),
		budget:     budget,
		logger:     logger,
	}
}

// Track records a single LLM API usage event.
func (t *UsageTracker) Track(ctx context.Context, providerName, model string, inputTokens, outputTokens int64, project string) (*UsageRecord, error) {
	cost, err := t.calculator.Calculate(providerName, model, inputTokens, outputTokens)
	if err != nil {
		return nil, fmt.Errorf("calculate cost: %w", err)
	}

	record := &UsageRecord{
		ID:           uuid.New().String(),
		Provider:     providerName,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      cost,
		Project:      project,
		Timestamp:    time.Now().UTC(),
	}

	if err := t.storage.RecordUsage(ctx, record); err != nil {
		return nil, fmt.Errorf("store usage: %w", err)
	}

	t.logger.Info("usage recorded",
		"provider", providerName,
		"model", model,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"cost_usd", cost,
		"project", project,
	)

	// Check budgets
	if t.budget != nil {
		if checkErr := t.budget.RecordSpend(ctx, record.CostUSD); checkErr != nil {
			t.logger.Error("budget check failed", "error", checkErr)
		}
	}

	return record, nil
}

// TrackWithTokens records usage with pre-calculated token counts (from API response).
func (t *UsageTracker) TrackWithTokens(ctx context.Context, record *UsageRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	// Calculate cost if not provided
	if record.CostUSD == 0 {
		cost, err := t.calculator.Calculate(record.Provider, record.Model, record.InputTokens, record.OutputTokens)
		if err != nil {
			return fmt.Errorf("calculate cost: %w", err)
		}
		record.CostUSD = cost
	}

	if err := t.storage.RecordUsage(ctx, record); err != nil {
		return fmt.Errorf("store usage: %w", err)
	}

	// Check budgets
	if t.budget != nil {
		if checkErr := t.budget.RecordSpend(ctx, record.CostUSD); checkErr != nil {
			t.logger.Error("budget check failed", "error", checkErr)
		}
	}

	return nil
}

// Report generates a usage summary for the given filter.
func (t *UsageTracker) Report(ctx context.Context, filter ReportFilter) (*UsageSummary, error) {
	return t.storage.AggregateUsage(ctx, filter)
}

// Query returns individual usage records for the given filter.
func (t *UsageTracker) Query(ctx context.Context, filter ReportFilter) ([]UsageRecord, error) {
	return t.storage.QueryUsage(ctx, filter)
}

// CheckBudget verifies if spending is within budget limits. Returns an error if any budget is exceeded.
func (t *UsageTracker) CheckBudget(ctx context.Context) error {
	if t.budget == nil {
		return nil
	}
	return t.budget.CheckAll(ctx)
}
