package store

import (
	"context"
	"database/sql"
)

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS sources (id TEXT PRIMARY KEY, type TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS source_cursors (source_id TEXT PRIMARY KEY REFERENCES sources(id), cursor TEXT NOT NULL, updated_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS inbox_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id TEXT NOT NULL,
			external_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			title TEXT NOT NULL,
			archived_at TEXT,
			ignored_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(source_id, external_id)
		);`,
		`CREATE TABLE IF NOT EXISTS inbox_item_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			inbox_item_id INTEGER NOT NULL REFERENCES inbox_items(id),
			version INTEGER NOT NULL,
			raw_payload TEXT NOT NULL,
			normalized TEXT NOT NULL,
			payload_hash TEXT NOT NULL,
			observed_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(inbox_item_id, version)
		);`,
		`CREATE TABLE IF NOT EXISTS workflow_definitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS workflow_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_definition_id INTEGER NOT NULL REFERENCES workflow_definitions(id),
			version INTEGER NOT NULL,
			hash TEXT NOT NULL,
			path TEXT NOT NULL,
			yaml TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(workflow_definition_id, version),
			UNIQUE(workflow_definition_id, hash)
		);`,
		`CREATE TABLE IF NOT EXISTS trigger_evaluations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			inbox_item_id INTEGER NOT NULL REFERENCES inbox_items(id),
			inbox_item_version_id INTEGER NOT NULL REFERENCES inbox_item_versions(id),
			workflow_definition_id INTEGER NOT NULL REFERENCES workflow_definitions(id),
			workflow_version_id INTEGER NOT NULL REFERENCES workflow_versions(id),
			matched INTEGER NOT NULL,
			policy TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS workflow_dispatches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			inbox_item_id INTEGER NOT NULL REFERENCES inbox_items(id),
			inbox_item_version_id INTEGER NOT NULL REFERENCES inbox_item_versions(id),
			workflow_id INTEGER NOT NULL REFERENCES workflow_definitions(id),
			workflow_version_id INTEGER NOT NULL REFERENCES workflow_versions(id),
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS workflow_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_dispatch_id INTEGER NOT NULL REFERENCES workflow_dispatches(id),
			workflow_version_id INTEGER NOT NULL REFERENCES workflow_versions(id),
			status TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT,
			error TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS step_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_run_id INTEGER NOT NULL REFERENCES workflow_runs(id),
			step_id TEXT NOT NULL,
			step_type TEXT NOT NULL,
			status TEXT NOT NULL,
			input_json TEXT NOT NULL,
			output_json TEXT NOT NULL,
			error TEXT,
			started_at TEXT,
			finished_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS retry_attempts (id INTEGER PRIMARY KEY AUTOINCREMENT, step_run_id INTEGER REFERENCES step_runs(id), attempt INTEGER NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			inbox_item_id INTEGER REFERENCES inbox_items(id),
			workflow_run_id INTEGER REFERENCES workflow_runs(id),
			step_run_id INTEGER REFERENCES step_runs(id),
			type TEXT NOT NULL,
			path TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sink_outputs (id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_run_id INTEGER NOT NULL REFERENCES workflow_runs(id), type TEXT NOT NULL, path TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS daemon_events (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL, message TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS secret_metadata (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL);`,
	}
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		for _, stmt := range stmts {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, ?)`, now())
		return err
	})
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
