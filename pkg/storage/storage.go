package storage

import (
	"context"

	"github.com/yapay-ai/llm-cost-guardian/pkg/model"
)

// Storage defines the persistence layer for usage records and budgets.
type Storage interface {
	// RecordUsage persists a single usage record.
	RecordUsage(ctx context.Context, record *model.UsageRecord) error

	// QueryUsage retrieves usage records matching the given filter.
	QueryUsage(ctx context.Context, filter model.ReportFilter) ([]model.UsageRecord, error)

	// AggregateUsage returns total cost and tokens for a time range.
	AggregateUsage(ctx context.Context, filter model.ReportFilter) (*model.UsageSummary, error)

	// SetBudget creates or updates a budget.
	SetBudget(ctx context.Context, budget *model.Budget) error

	// GetBudget retrieves a budget by name.
	GetBudget(ctx context.Context, name string) (*model.Budget, error)

	// ListBudgets returns all configured budgets.
	ListBudgets(ctx context.Context) ([]model.Budget, error)

	// UpdateBudgetSpend atomically updates the current spend for a budget.
	UpdateBudgetSpend(ctx context.Context, name string, amount float64) error

	// Close releases resources.
	Close() error
}
