package tracker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/yapay-ai/llm-cost-guardian/pkg/alerts"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
)

// BudgetManager handles budget checking and alert dispatching.
type BudgetManager struct {
	storage   storage.Storage
	notifiers []alerts.Notifier
	logger    *slog.Logger
}

// NewBudgetManager creates a budget manager.
func NewBudgetManager(store storage.Storage, notifiers []alerts.Notifier, logger *slog.Logger) *BudgetManager {
	return &BudgetManager{
		storage:   store,
		notifiers: notifiers,
		logger:    logger,
	}
}

// RecordSpend adds the given amount to all active budgets and checks thresholds.
func (m *BudgetManager) RecordSpend(ctx context.Context, amount float64) error {
	budgets, err := m.storage.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("list budgets: %w", err)
	}

	for _, budget := range budgets {
		if err := m.storage.UpdateBudgetSpend(ctx, budget.Name, amount); err != nil {
			m.logger.Error("update budget spend", "budget", budget.Name, "error", err)
			continue
		}

		// Re-read to get updated spend
		updated, err := m.storage.GetBudget(ctx, budget.Name)
		if err != nil {
			m.logger.Error("get updated budget", "budget", budget.Name, "error", err)
			continue
		}

		m.checkThresholds(ctx, updated)
	}

	return nil
}

// CheckAll checks all budgets against their thresholds.
func (m *BudgetManager) CheckAll(ctx context.Context) error {
	budgets, err := m.storage.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("list budgets: %w", err)
	}

	for _, budget := range budgets {
		if budget.CurrentSpend >= budget.LimitUSD {
			return fmt.Errorf("budget %q exceeded: $%.2f / $%.2f", budget.Name, budget.CurrentSpend, budget.LimitUSD)
		}
	}

	return nil
}

// checkThresholds evaluates a budget and dispatches alerts if thresholds are crossed.
func (m *BudgetManager) checkThresholds(ctx context.Context, budget *Budget) {
	if budget.LimitUSD <= 0 {
		return
	}

	pct := (budget.CurrentSpend / budget.LimitUSD) * 100

	var level alerts.AlertLevel
	switch {
	case pct >= 100:
		level = alerts.AlertExceeded
	case pct >= 95:
		level = alerts.AlertCritical
	case pct >= budget.AlertThresholdPct:
		level = alerts.AlertWarning
	default:
		return // Under threshold, no alert needed
	}

	alert := alerts.Alert{
		Level:        level,
		BudgetName:   budget.Name,
		LimitUSD:     budget.LimitUSD,
		CurrentSpend: budget.CurrentSpend,
		ThresholdPct: budget.AlertThresholdPct,
		Period:       string(budget.Period),
		Message: fmt.Sprintf("Budget %q at %.1f%% ($%.2f / $%.2f)",
			budget.Name, pct, budget.CurrentSpend, budget.LimitUSD),
	}

	m.logger.Warn("budget threshold crossed",
		"budget", budget.Name,
		"level", level,
		"pct", pct,
		"spend", budget.CurrentSpend,
		"limit", budget.LimitUSD,
	)

	for _, notifier := range m.notifiers {
		if err := notifier.Send(ctx, alert); err != nil {
			m.logger.Error("send alert failed",
				"notifier", notifier.Name(),
				"budget", budget.Name,
				"error", err,
			)
		}
	}
}

// ResetBudgetSpend resets the current spend for a budget (used for period rollovers).
func (m *BudgetManager) ResetBudgetSpend(ctx context.Context, name string) error {
	budget, err := m.storage.GetBudget(ctx, name)
	if err != nil {
		return err
	}

	// Reset by subtracting current spend
	return m.storage.UpdateBudgetSpend(ctx, name, -budget.CurrentSpend)
}
