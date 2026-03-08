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

const defaultTenantSlug = "default"

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

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	store := &SQLite{db: db}
	if _, err := store.EnsureTenant(context.Background(), defaultTenantSlug, "Default"); err != nil {
		db.Close()
		return nil, fmt.Errorf("ensure default tenant: %w", err)
	}

	return store, nil
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

	tenant, err := s.resolveTenant(ctx, record.TenantID, record.Tenant)
	if err != nil {
		return err
	}
	record.TenantID = tenant.ID
	record.Tenant = tenant.Slug

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO usage_records (id, tenant_id, provider, model, input_tokens, output_tokens, cost_usd, project, metadata, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.TenantID, record.Provider, record.Model,
		record.InputTokens, record.OutputTokens, record.CostUSD,
		record.Project, record.Metadata, record.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert usage record: %w", err)
	}

	if err := s.recordUsageRollup(ctx, record, "hourly"); err != nil {
		return err
	}
	if err := s.recordUsageRollup(ctx, record, "daily"); err != nil {
		return err
	}

	return nil
}

func (s *SQLite) QueryUsage(ctx context.Context, filter model.ReportFilter) ([]model.UsageRecord, error) {
	query := `SELECT u.id, u.tenant_id, t.slug, u.provider, u.model, u.input_tokens, u.output_tokens, u.cost_usd, u.project, u.metadata, u.timestamp
		FROM usage_records u
		JOIN tenants t ON u.tenant_id = t.id`
	where, args := buildWhereClause(filter, "u", "t")
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY u.timestamp DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage: %w", err)
	}
	defer rows.Close()

	var records []model.UsageRecord
	for rows.Next() {
		var r model.UsageRecord
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Tenant, &r.Provider, &r.Model, &r.InputTokens, &r.OutputTokens,
			&r.CostUSD, &r.Project, &r.Metadata, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("scan usage row: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *SQLite) AggregateUsage(ctx context.Context, filter model.ReportFilter) (*model.UsageSummary, error) {
	query := `SELECT
		COALESCE(SUM(u.cost_usd), 0),
		COALESCE(SUM(u.input_tokens), 0),
		COALESCE(SUM(u.output_tokens), 0),
		COUNT(*)
	FROM usage_records u
	JOIN tenants t ON u.tenant_id = t.id`
	where, args := buildWhereClause(filter, "u", "t")
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

	summary.ByTenant, err = s.aggregateByField(ctx, "t.slug", filter)
	if err != nil {
		return nil, err
	}
	summary.ByProvider, err = s.aggregateByField(ctx, "u.provider", filter)
	if err != nil {
		return nil, err
	}
	summary.ByModel, err = s.aggregateByField(ctx, "u.model", filter)
	if err != nil {
		return nil, err
	}
	summary.ByProject, err = s.aggregateByField(ctx, "u.project", filter)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (s *SQLite) aggregateByField(ctx context.Context, field string, filter model.ReportFilter) (map[string]float64, error) {
	query := fmt.Sprintf(`SELECT %s, COALESCE(SUM(u.cost_usd), 0)
		FROM usage_records u
		JOIN tenants t ON u.tenant_id = t.id`, field)
	where, args := buildWhereClause(filter, "u", "t")
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
		var name sql.NullString
		var total float64
		if err := rows.Scan(&name, &total); err != nil {
			return nil, fmt.Errorf("scan %s aggregate: %w", field, err)
		}
		key := ""
		if name.Valid {
			key = name.String
		}
		result[key] = total
	}
	return result, rows.Err()
}

func (s *SQLite) SetBudget(ctx context.Context, budget *model.Budget) error {
	if budget.ID == "" {
		budget.ID = uuid.New().String()
	}
	budget.Project = strings.TrimSpace(budget.Project)
	now := time.Now().UTC()
	if budget.CreatedAt.IsZero() {
		budget.CreatedAt = now
	}
	budget.UpdatedAt = now

	tenant, err := s.resolveTenant(ctx, budget.TenantID, budget.Tenant)
	if err != nil {
		return err
	}
	budget.TenantID = tenant.ID
	budget.Tenant = tenant.Slug

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO budgets (id, tenant_id, name, project, limit_usd, period, current_spend, alert_threshold_pct, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   tenant_id = excluded.tenant_id,
		   project = excluded.project,
		   limit_usd = excluded.limit_usd,
		   period = excluded.period,
		   alert_threshold_pct = excluded.alert_threshold_pct,
		   updated_at = excluded.updated_at`,
		budget.ID, budget.TenantID, budget.Name, budget.Project, budget.LimitUSD, budget.Period,
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
		`SELECT b.id, b.tenant_id, t.slug, b.name, b.project, b.limit_usd, b.period, b.current_spend, b.alert_threshold_pct, b.created_at, b.updated_at
		 FROM budgets b
		 JOIN tenants t ON b.tenant_id = t.id
		 WHERE b.name = ?`, name,
	).Scan(&b.ID, &b.TenantID, &b.Tenant, &b.Name, &b.Project, &b.LimitUSD, &b.Period, &b.CurrentSpend,
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
		`SELECT b.id, b.tenant_id, t.slug, b.name, b.project, b.limit_usd, b.period, b.current_spend, b.alert_threshold_pct, b.created_at, b.updated_at
		 FROM budgets b
		 JOIN tenants t ON b.tenant_id = t.id
		 ORDER BY t.slug, b.name`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	var budgets []model.Budget
	for rows.Next() {
		var b model.Budget
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Tenant, &b.Name, &b.Project, &b.LimitUSD, &b.Period, &b.CurrentSpend,
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

func (s *SQLite) EnsureTenant(ctx context.Context, slug, name string) (*model.Tenant, error) {
	slug = normalizeTenantSlug(slug)
	if slug == "" {
		slug = defaultTenantSlug
	}
	if name == "" {
		name = tenantNameFromSlug(slug)
	}

	tenant, err := s.GetTenant(ctx, slug)
	if err == nil {
		return tenant, nil
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, slug, name, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, slug, name, model.TenantStatusActive, now, now,
	); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return s.GetTenant(ctx, slug)
		}
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	return &model.Tenant{
		ID:        id,
		Slug:      slug,
		Name:      name,
		Status:    model.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *SQLite) CreateTenant(ctx context.Context, tenant *model.Tenant) error {
	tenant.Slug = normalizeTenantSlug(tenant.Slug)
	if tenant.Slug == "" {
		return fmt.Errorf("tenant slug is required")
	}
	if tenant.Name == "" {
		tenant.Name = tenantNameFromSlug(tenant.Slug)
	}
	if tenant.Status == "" {
		tenant.Status = model.TenantStatusActive
	}
	if tenant.ID == "" {
		tenant.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if tenant.CreatedAt.IsZero() {
		tenant.CreatedAt = now
	}
	tenant.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, slug, name, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		tenant.ID, tenant.Slug, tenant.Name, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (s *SQLite) GetTenant(ctx context.Context, slug string) (*model.Tenant, error) {
	var tenant model.Tenant
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, status, created_at, updated_at FROM tenants WHERE slug = ?`,
		normalizeTenantSlug(slug),
	).Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tenant %q not found", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return &tenant, nil
}

func (s *SQLite) ListTenants(ctx context.Context) ([]model.Tenant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, name, status, created_at, updated_at FROM tenants ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var tenant model.Tenant
		if err := rows.Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant row: %w", err)
		}
		tenants = append(tenants, tenant)
	}
	return tenants, rows.Err()
}

func (s *SQLite) DisableTenant(ctx context.Context, slug string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET status = ?, updated_at = ? WHERE slug = ?`,
		model.TenantStatusDisabled, time.Now().UTC(), normalizeTenantSlug(slug),
	)
	if err != nil {
		return fmt.Errorf("disable tenant: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("tenant %q not found", slug)
	}
	return nil
}

func (s *SQLite) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	if key.ID == "" {
		key.ID = uuid.New().String()
	}
	if strings.TrimSpace(key.KeyHash) == "" {
		return fmt.Errorf("api key hash is required")
	}

	tenant, err := s.resolveTenant(ctx, key.TenantID, key.Tenant)
	if err != nil {
		return err
	}
	key.TenantID = tenant.ID
	key.Tenant = tenant.Slug
	if key.Status == "" {
		key.Status = model.APIKeyStatusActive
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now().UTC()
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, tenant_id, name, key_prefix, key_hash, status, last_used_at, created_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.TenantID, key.Name, key.KeyPrefix, key.KeyHash, key.Status, key.LastUsedAt, key.CreatedAt, key.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *SQLite) ListAPIKeys(ctx context.Context, tenant string) ([]model.APIKey, error) {
	query := `SELECT k.id, k.tenant_id, t.slug, k.name, k.key_prefix, k.key_hash, k.status, k.last_used_at, k.created_at, k.revoked_at
		FROM api_keys k
		JOIN tenants t ON k.tenant_id = t.id`
	var args []any
	if tenant = normalizeTenantSlug(tenant); tenant != "" {
		query += " WHERE t.slug = ?"
		args = append(args, tenant)
	}
	query += " ORDER BY t.slug, k.name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		var key model.APIKey
		if err := rows.Scan(&key.ID, &key.TenantID, &key.Tenant, &key.Name, &key.KeyPrefix, &key.KeyHash, &key.Status, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan api key row: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *SQLite) RevokeAPIKey(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET status = ?, revoked_at = ? WHERE id = ?`,
		model.APIKeyStatusRevoked, now, id,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("api key %q not found", id)
	}
	return nil
}

func (s *SQLite) ResolveAPIKey(ctx context.Context, keyHash string) (*model.APIKey, *model.Tenant, error) {
	var key model.APIKey
	var tenant model.Tenant
	err := s.db.QueryRowContext(ctx,
		`SELECT k.id, k.tenant_id, t.slug, k.name, k.key_prefix, k.key_hash, k.status, k.last_used_at, k.created_at, k.revoked_at,
		        t.id, t.slug, t.name, t.status, t.created_at, t.updated_at
		 FROM api_keys k
		 JOIN tenants t ON k.tenant_id = t.id
		 WHERE k.key_hash = ? AND k.status = ? AND t.status = ?`,
		keyHash, model.APIKeyStatusActive, model.TenantStatusActive,
	).Scan(
		&key.ID, &key.TenantID, &key.Tenant, &key.Name, &key.KeyPrefix, &key.KeyHash, &key.Status, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt,
		&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("api key not found")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("resolve api key: %w", err)
	}

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, now, key.ID); err != nil {
		return nil, nil, fmt.Errorf("touch api key: %w", err)
	}
	key.LastUsedAt = &now
	return &key, &tenant, nil
}

func (s *SQLite) QueryUsageRollups(ctx context.Context, filter model.ReportFilter, granularity string, start, end time.Time) ([]model.UsageRollup, error) {
	query := `SELECT t.slug, r.provider, r.model, r.project, r.granularity, r.bucket_start, r.request_count, r.input_tokens, r.output_tokens, r.cost_usd
		FROM usage_rollups r
		JOIN tenants t ON r.tenant_id = t.id
		WHERE r.granularity = ?`
	args := []any{granularity}

	if filter.Tenant != "" {
		query += " AND t.slug = ?"
		args = append(args, normalizeTenantSlug(filter.Tenant))
	}
	if filter.Provider != "" {
		query += " AND r.provider = ?"
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		query += " AND r.model = ?"
		args = append(args, filter.Model)
	}
	if filter.Project != "" {
		query += " AND r.project = ?"
		args = append(args, filter.Project)
	}
	if !start.IsZero() {
		query += " AND r.bucket_start >= ?"
		args = append(args, start)
	}
	if !end.IsZero() {
		query += " AND r.bucket_start < ?"
		args = append(args, end)
	}
	query += " ORDER BY r.bucket_start ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage rollups: %w", err)
	}
	defer rows.Close()

	var rollups []model.UsageRollup
	for rows.Next() {
		var rollup model.UsageRollup
		if err := rows.Scan(&rollup.Tenant, &rollup.Provider, &rollup.Model, &rollup.Project, &rollup.Granularity, &rollup.BucketStart, &rollup.RequestCount, &rollup.InputTokens, &rollup.OutputTokens, &rollup.CostUSD); err != nil {
			return nil, fmt.Errorf("scan usage rollup: %w", err)
		}
		rollups = append(rollups, rollup)
	}
	return rollups, rows.Err()
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) resolveTenant(ctx context.Context, tenantID, tenantSlug string) (*model.Tenant, error) {
	if strings.TrimSpace(tenantID) != "" {
		var tenant model.Tenant
		err := s.db.QueryRowContext(ctx,
			`SELECT id, slug, name, status, created_at, updated_at FROM tenants WHERE id = ?`,
			tenantID,
		).Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt)
		if err == nil {
			return &tenant, nil
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("get tenant by id: %w", err)
		}
	}

	return s.EnsureTenant(ctx, tenantSlug, tenantNameFromSlug(tenantSlug))
}

func (s *SQLite) recordUsageRollup(ctx context.Context, record *model.UsageRecord, granularity string) error {
	bucketStart := truncateBucket(record.Timestamp, granularity)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage_rollups (tenant_id, granularity, bucket_start, provider, model, project, request_count, input_tokens, output_tokens, cost_usd)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?)
		 ON CONFLICT(tenant_id, granularity, bucket_start, provider, model, project) DO UPDATE SET
		   request_count = usage_rollups.request_count + 1,
		   input_tokens = usage_rollups.input_tokens + excluded.input_tokens,
		   output_tokens = usage_rollups.output_tokens + excluded.output_tokens,
		   cost_usd = usage_rollups.cost_usd + excluded.cost_usd`,
		record.TenantID, granularity, bucketStart, record.Provider, record.Model, record.Project,
		record.InputTokens, record.OutputTokens, record.CostUSD,
	)
	if err != nil {
		return fmt.Errorf("record %s rollup: %w", granularity, err)
	}
	return nil
}

func buildWhereClause(filter model.ReportFilter, usageAlias, tenantAlias string) (string, []any) {
	var conditions []string
	var args []any

	if filter.Tenant != "" {
		conditions = append(conditions, tenantAlias+".slug = ?")
		args = append(args, normalizeTenantSlug(filter.Tenant))
	}
	if filter.Provider != "" {
		conditions = append(conditions, usageAlias+".provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, usageAlias+".model = ?")
		args = append(args, filter.Model)
	}
	if filter.Project != "" {
		conditions = append(conditions, usageAlias+".project = ?")
		args = append(args, filter.Project)
	}
	if !filter.StartTime.IsZero() {
		conditions = append(conditions, usageAlias+".timestamp >= ?")
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		conditions = append(conditions, usageAlias+".timestamp < ?")
		args = append(args, filter.EndTime)
	}

	return strings.Join(conditions, " AND "), args
}

func normalizeTenantSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = strings.ReplaceAll(slug, " ", "-")
	if slug == "" {
		return ""
	}
	return slug
}

func tenantNameFromSlug(slug string) string {
	slug = normalizeTenantSlug(slug)
	if slug == "" {
		return "Default"
	}
	parts := strings.Split(slug, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func truncateBucket(ts time.Time, granularity string) time.Time {
	ts = ts.UTC()
	switch granularity {
	case "hourly":
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, time.UTC)
	default:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
	}
}
