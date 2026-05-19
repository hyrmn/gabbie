package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyMigrations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Find migrations dir relative to project root
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	migrationsPath := filepath.Join(projectRoot, "migrations")

	// Create temp file
	f, err := os.CreateTemp("", "todotracker-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	dbConn, err := sql.Open("sqlite", f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()

	// Enable WAL and foreign keys
	dbConn.Exec("PRAGMA journal_mode=WAL")
	dbConn.Exec("PRAGMA foreign_keys=ON")

	// Read and execute migrations
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(migrationsPath, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if _, err := dbConn.Exec(string(data)); err != nil {
			t.Fatalf("execute %s: %v", e.Name(), err)
		}
	}

	// Verify all tables exist
	tables := []string{"users", "lists", "list_collaborators", "items", "api_keys"}
	for _, table := range tables {
		var count int
		err := dbConn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Errorf("query sqlite_master: %v", err)
			continue
		}
		if count == 0 {
			t.Errorf("table %s does not exist", table)
		}
	}

	// Verify indexes
	var idxCount int
	err = dbConn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'").Scan(&idxCount)
	if err != nil {
		t.Errorf("count indexes: %v", err)
	}
	if idxCount != 6 {
		t.Errorf("expected 6 indexes, got %d", idxCount)
	}

	logger.Info("Migration test passed", "tables", len(tables), "indexes", idxCount)
	fmt.Println("✓ All migrations applied successfully")
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		filename string
		want     int
	}{
		{"001_initial.sql", 1},
		{"010_add_indexes.sql", 10},
		{"999_migration.sql", 999},
		{"invalid.sql", 0},
	}
	for _, tt := range tests {
		got := parseVersion(tt.filename)
		if got != tt.want {
			t.Errorf("parseVersion(%q) = %d, want %d", tt.filename, got, tt.want)
		}
	}
}
