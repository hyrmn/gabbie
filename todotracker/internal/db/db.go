// Package db provides SQLite database connection and migration management.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps the sql.DB with application-specific helpers.
type DB struct {
	*sql.DB
	Logger *slog.Logger
}

// New opens a SQLite database at the given path, enables WAL mode,
// and applies pending migrations.
func New(dbPath string, migrationsFS embed.FS, logger *slog.Logger) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	d := &DB{DB: db, Logger: logger}

	// Enable WAL mode for better concurrency
	if _, err := d.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := d.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := d.ApplyMigrations(migrationsFS); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return d, nil
}

// ApplyMigrations reads SQL files from the embedded migrations directory
// and executes them in alphabetical order within a transaction.
func (d *DB) ApplyMigrations(migrationsFS embed.FS) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Ensure migrations tracking table exists
	if _, err := d.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for _, f := range files {
		// Extract version number from filename (e.g., "001_initial.sql" -> 1)
		version := parseVersion(f)
		if version == 0 {
			d.Logger.Warn("skipping migration with invalid version", "file", f)
			continue
		}

		// Check if already applied
		var exists bool
		err := d.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", version).Scan(&exists)
		if err == nil {
			d.Logger.Debug("migration already applied", "version", version)
			continue
		}

		d.Logger.Info("applying migration", "version", version, "file", f)
		data, err := migrationsFS.ReadFile(path.Join("migrations", f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		// Execute migration in a transaction
		if err := d.applyMigrationTx(version, string(data)); err != nil {
			return fmt.Errorf("execute migration %s: %w", f, err)
		}
	}

	return nil
}

// applyMigrationTx runs a single migration within a transaction.
func (d *DB) applyMigrationTx(version int, sql string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sql); err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// parseVersion extracts the integer version from a migration filename.
// Expected format: "NNN_description.sql" -> NNN as integer.
func parseVersion(filename string) int {
	base := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 0 {
		return 0
	}

	var version int
	fmt.Sscanf(parts[0], "%d", &version)
	return version
}
