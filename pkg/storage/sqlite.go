package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"

	_ "modernc.org/sqlite"
)

// SQLite implements the Storage interface using an SQLite database.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens or creates an SQLite database at the given path.
func NewSQLite(dbPath string) (*SQLite, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &SQLite{db: db}, nil
}

func (s *SQLite) RecordUsage(ctx context.Context, record *model.UsageRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}
	if record.Metadata == "" {
		record.Metadata = "{}"
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage_records (id, provider, model, input_tokens, output_tokens, cost_usd, project, metadata, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.Provider, record.Model,
		record.InputTokens, record.OutputTokens, record.CostUSD,
		record.Project, record.Metadata, record.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert usage record: %w", err)
	}
	return nil
}

func (s *SQLite) QueryUsage(ctx context.Context, filter model.ReportFilter) ([]model.UsageRecord, error) {
	query := "SELECT id, provider, model, input_tokens, output_tokens, cost_usd, project, metadata, timestamp FROM usage_records"
	where, args := buildWhereClause(filter)
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY timestamp DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage: %w", err)
	}
	defer rows.Close()

	var records []model.UsageRecord
	for rows.Next() {
		var r model.UsageRecord
		if err := rows.Scan(&r.ID, &r.Provider, &r.Model, &r.InputTokens, &r.OutputTokens,
			&r.CostUSD, &r.Project, &r.Metadata, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("scan usage row: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *SQLite) AggregateUsage(ctx context.Context, filter model.ReportFilter) (*model.UsageSummary, error) {
	query := `SELECT
		COALESCE(SUM(cost_usd), 0),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COUNT(*)
	FROM usage_records`
	where, args := buildWhereClause(filter)
	if where != "" {
		query += " WHERE " + where
	}

	summary := &model.UsageSummary{}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.TotalCostUSD,
		&summary.TotalInputTokens,
		&summary.TotalOutputTokens,
		&summary.RecordCount,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate usage: %w", err)
	}

	// Aggregate by provider
	summary.ByProvider, err = s.aggregateByField(ctx, "provider", where, args)
	if err != nil {
		return nil, err
	}

	// Aggregate by model
	summary.ByModel, err = s.aggregateByField(ctx, "model", where, args)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (s *SQLite) aggregateByField(ctx context.Context, field, where string, args []any) (map[string]float64, error) {
	query := fmt.Sprintf("SELECT %s, COALESCE(SUM(cost_usd), 0) FROM usage_records", field)
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" GROUP BY %s", field)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("aggregate by %s: %w", field, err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var name string
		var total float64
		if err := rows.Scan(&name, &total); err != nil {
			return nil, fmt.Errorf("scan %s aggregate: %w", field, err)
		}
		result[name] = total
	}
	return result, rows.Err()
}

func (s *SQLite) SetBudget(ctx context.Context, budget *model.Budget) error {
	if budget.ID == "" {
		budget.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if budget.CreatedAt.IsZero() {
		budget.CreatedAt = now
	}
	budget.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO budgets (id, name, limit_usd, period, current_spend, alert_threshold_pct, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   limit_usd = excluded.limit_usd,
		   period = excluded.period,
		   alert_threshold_pct = excluded.alert_threshold_pct,
		   updated_at = excluded.updated_at`,
		budget.ID, budget.Name, budget.LimitUSD, budget.Period,
		budget.CurrentSpend, budget.AlertThresholdPct, budget.CreatedAt, budget.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("set budget: %w", err)
	}
	return nil
}

func (s *SQLite) GetBudget(ctx context.Context, name string) (*model.Budget, error) {
	var b model.Budget
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, limit_usd, period, current_spend, alert_threshold_pct, created_at, updated_at
		 FROM budgets WHERE name = ?`, name,
	).Scan(&b.ID, &b.Name, &b.LimitUSD, &b.Period, &b.CurrentSpend,
		&b.AlertThresholdPct, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("budget %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("get budget: %w", err)
	}
	return &b, nil
}

func (s *SQLite) ListBudgets(ctx context.Context) ([]model.Budget, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, limit_usd, period, current_spend, alert_threshold_pct, created_at, updated_at
		 FROM budgets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	var budgets []model.Budget
	for rows.Next() {
		var b model.Budget
		if err := rows.Scan(&b.ID, &b.Name, &b.LimitUSD, &b.Period, &b.CurrentSpend,
			&b.AlertThresholdPct, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan budget row: %w", err)
		}
		budgets = append(budgets, b)
	}
	return budgets, rows.Err()
}

func (s *SQLite) UpdateBudgetSpend(ctx context.Context, name string, amount float64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE budgets SET current_spend = current_spend + ?, updated_at = ? WHERE name = ?`,
		amount, time.Now().UTC(), name,
	)
	if err != nil {
		return fmt.Errorf("update budget spend: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("budget %q not found", name)
	}
	return nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// buildWhereClause constructs a SQL WHERE clause from a ReportFilter.
func buildWhereClause(filter model.ReportFilter) (string, []any) {
	var conditions []string
	var args []any

	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, filter.Project)
	}
	if !filter.StartTime.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		conditions = append(conditions, "timestamp < ?")
		args = append(args, filter.EndTime)
	}

	return strings.Join(conditions, " AND "), args
}
