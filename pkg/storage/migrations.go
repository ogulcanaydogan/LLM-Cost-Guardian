package storage

import (
	"database/sql"
	"fmt"
)

var migrations = []string{
	// Migration 1: Initial schema
	`CREATE TABLE IF NOT EXISTS usage_records (
		id            TEXT PRIMARY KEY,
		provider      TEXT NOT NULL,
		model         TEXT NOT NULL,
		input_tokens  INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd      REAL NOT NULL DEFAULT 0.0,
		project       TEXT NOT NULL DEFAULT 'default',
		metadata      TEXT DEFAULT '{}',
		timestamp     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_usage_provider ON usage_records(provider);
	CREATE INDEX IF NOT EXISTS idx_usage_project ON usage_records(project);
	CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage_records(timestamp);
	CREATE INDEX IF NOT EXISTS idx_usage_model ON usage_records(model);

	CREATE TABLE IF NOT EXISTS budgets (
		id                  TEXT PRIMARY KEY,
		name                TEXT NOT NULL UNIQUE,
		limit_usd           REAL NOT NULL,
		period              TEXT NOT NULL CHECK(period IN ('daily', 'weekly', 'monthly')),
		current_spend       REAL NOT NULL DEFAULT 0.0,
		alert_threshold_pct REAL NOT NULL DEFAULT 80.0,
		created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
	// Migration 2: Add project scope to budgets.
	`ALTER TABLE budgets ADD COLUMN project TEXT NOT NULL DEFAULT '';

	CREATE INDEX IF NOT EXISTS idx_budgets_project ON budgets(project);`,
	// Migration 3: Add tenants, API keys, tenant scoping, and analytics rollups.
	`CREATE TABLE IF NOT EXISTS tenants (
		id         TEXT PRIMARY KEY,
		slug       TEXT NOT NULL UNIQUE,
		name       TEXT NOT NULL,
		status     TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'disabled')),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	INSERT OR IGNORE INTO tenants (id, slug, name, status)
	VALUES ('default-tenant', 'default', 'Default', 'active');

	CREATE TABLE IF NOT EXISTS api_keys (
		id           TEXT PRIMARY KEY,
		tenant_id    TEXT NOT NULL,
		name         TEXT NOT NULL,
		key_prefix   TEXT NOT NULL,
		key_hash     TEXT NOT NULL UNIQUE,
		status       TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'revoked')),
		last_used_at DATETIME,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		revoked_at   DATETIME,
		FOREIGN KEY(tenant_id) REFERENCES tenants(id)
	);

	CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);

	ALTER TABLE usage_records ADD COLUMN tenant_id TEXT NOT NULL DEFAULT '';
	UPDATE usage_records
	SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
	WHERE COALESCE(tenant_id, '') = '';
	CREATE INDEX IF NOT EXISTS idx_usage_tenant ON usage_records(tenant_id);

	ALTER TABLE budgets ADD COLUMN tenant_id TEXT NOT NULL DEFAULT '';
	UPDATE budgets
	SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
	WHERE COALESCE(tenant_id, '') = '';
	CREATE INDEX IF NOT EXISTS idx_budgets_tenant ON budgets(tenant_id);

	CREATE TABLE IF NOT EXISTS usage_rollups (
		tenant_id     TEXT NOT NULL,
		granularity   TEXT NOT NULL CHECK(granularity IN ('hourly', 'daily')),
		bucket_start  DATETIME NOT NULL,
		provider      TEXT NOT NULL,
		model         TEXT NOT NULL,
		project       TEXT NOT NULL DEFAULT '',
		request_count INTEGER NOT NULL DEFAULT 0,
		input_tokens  INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd      REAL NOT NULL DEFAULT 0.0,
		PRIMARY KEY (tenant_id, granularity, bucket_start, provider, model, project),
		FOREIGN KEY(tenant_id) REFERENCES tenants(id)
	);

	INSERT INTO usage_rollups (tenant_id, granularity, bucket_start, provider, model, project, request_count, input_tokens, output_tokens, cost_usd)
	SELECT
		tenant_id,
		'daily' AS granularity,
		DATETIME(STRFTIME('%Y-%m-%d 00:00:00', timestamp)) AS bucket_start,
		provider,
		model,
		project,
		COUNT(*) AS request_count,
		COALESCE(SUM(input_tokens), 0) AS input_tokens,
		COALESCE(SUM(output_tokens), 0) AS output_tokens,
		COALESCE(SUM(cost_usd), 0) AS cost_usd
	FROM usage_records
	GROUP BY tenant_id, provider, model, project, STRFTIME('%Y-%m-%d', timestamp)
	ON CONFLICT(tenant_id, granularity, bucket_start, provider, model, project) DO UPDATE SET
		request_count = excluded.request_count,
		input_tokens = excluded.input_tokens,
		output_tokens = excluded.output_tokens,
		cost_usd = excluded.cost_usd;

	INSERT INTO usage_rollups (tenant_id, granularity, bucket_start, provider, model, project, request_count, input_tokens, output_tokens, cost_usd)
	SELECT
		tenant_id,
		'hourly' AS granularity,
		DATETIME(STRFTIME('%Y-%m-%d %H:00:00', timestamp)) AS bucket_start,
		provider,
		model,
		project,
		COUNT(*) AS request_count,
		COALESCE(SUM(input_tokens), 0) AS input_tokens,
		COALESCE(SUM(output_tokens), 0) AS output_tokens,
		COALESCE(SUM(cost_usd), 0) AS cost_usd
	FROM usage_records
	GROUP BY tenant_id, provider, model, project, STRFTIME('%Y-%m-%d %H', timestamp)
	ON CONFLICT(tenant_id, granularity, bucket_start, provider, model, project) DO UPDATE SET
		request_count = excluded.request_count,
		input_tokens = excluded.input_tokens,
		output_tokens = excluded.output_tokens,
		cost_usd = excluded.cost_usd;

	CREATE INDEX IF NOT EXISTS idx_rollups_bucket ON usage_rollups(granularity, bucket_start);
	CREATE INDEX IF NOT EXISTS idx_rollups_tenant ON usage_rollups(tenant_id);`,
}

// runMigrations applies pending schema migrations.
func runMigrations(db *sql.DB) error {
	// Ensure migration tracking table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("check migration version: %w", err)
	}

	for i := currentVersion; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", i+1, err)
		}

		if _, err := tx.Exec(migrations[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("run migration %d: %w", i+1, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", i+1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}

	return nil
}
