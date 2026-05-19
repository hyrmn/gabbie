// Package db provides SQLite database connection and query helpers.
package db

import "time"

const (
	sqliteTimeFormat = "2006-01-02 15:04:05"
)

// parseSQLiteTime parses a SQLite datetime string into time.Time.
// Handles both "2006-01-02 15:04:05" and RFC3339 formats.
func parseSQLiteTime(s string) (time.Time, error) {
	if t, err := time.Parse(sqliteTimeFormat, s); err == nil {
		return t.UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}
