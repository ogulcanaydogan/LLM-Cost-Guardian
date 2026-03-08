package storage

import (
	"context"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
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

	// EnsureTenant guarantees a tenant exists and returns it.
	EnsureTenant(ctx context.Context, slug, name string) (*model.Tenant, error)

	// CreateTenant creates a new tenant.
	CreateTenant(ctx context.Context, tenant *model.Tenant) error

	// GetTenant retrieves a tenant by slug.
	GetTenant(ctx context.Context, slug string) (*model.Tenant, error)

	// ListTenants returns all configured tenants.
	ListTenants(ctx context.Context) ([]model.Tenant, error)

	// DisableTenant disables a tenant.
	DisableTenant(ctx context.Context, slug string) error

	// CreateAPIKey stores a hashed API key.
	CreateAPIKey(ctx context.Context, key *model.APIKey) error

	// ListAPIKeys returns API keys, optionally filtered by tenant slug.
	ListAPIKeys(ctx context.Context, tenant string) ([]model.APIKey, error)

	// RevokeAPIKey revokes a key by id.
	RevokeAPIKey(ctx context.Context, id string) error

	// ResolveAPIKey returns an active API key and tenant by hash and updates last_used_at.
	ResolveAPIKey(ctx context.Context, keyHash string) (*model.APIKey, *model.Tenant, error)

	// QueryUsageRollups returns aggregated hourly or daily usage buckets.
	QueryUsageRollups(ctx context.Context, filter model.ReportFilter, granularity string, start, end time.Time) ([]model.UsageRollup, error)

	// Close releases resources.
	Close() error
}
