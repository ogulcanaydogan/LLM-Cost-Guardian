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
