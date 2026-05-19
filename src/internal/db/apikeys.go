// Package db provides SQLite database connection and query helpers.
package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/hyrmn/todotracker/internal/models"
)

const (
	// APIKeyPrefix is the human-readable prefix for generated API keys.
	APIKeyPrefix = "ctx7sk"
	// APIKeyRandomBytes is the number of random bytes used for key generation
	// (32 bytes = 64 hex characters).
	APIKeyRandomBytes = 32
)

// GenerateAPIKey creates a new API key and returns:
// - prefix: first 8 characters for display (e.g., "ctx7sk-9a")
// - rawKey: the full key to show to the user ONCE at creation
// - hash: SHA-256 hash of the full key for storage
func GenerateAPIKey() (prefix, rawKey, hash string) {
	// Generate random bytes
	randomBytes := make([]byte, APIKeyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		// This should never happen in practice
		panic("crypto/rand.Read failed: " + err.Error())
	}

	rawKey = APIKeyPrefix + "-" + hex.EncodeToString(randomBytes)
	prefix = rawKey[:8]

	// Hash the full key for storage
	h := sha256.Sum256([]byte(rawKey))
	hash = hex.EncodeToString(h[:])

	return prefix, rawKey, hash
}

// CreateAPIKey inserts a new API key into the database.
func (d *DB) CreateAPIKey(ctx context.Context, userID, name, keyHash, keyPrefix string) (*models.APIKey, error) {
	id := uuid.New().String()

	_, err := d.ExecContext(ctx, `
		INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, created_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, id, userID, name, keyHash, keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	return d.GetAPIKey(ctx, id)
}

// GetAPIKey returns a single API key by ID.
func (d *DB) GetAPIKey(ctx context.Context, id string) (*models.APIKey, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, user_id, name, key_hash, key_prefix, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE id = ?
	`, id)

	return d.scanAPIKey(row)
}

// GetAPIKeysByUser returns all API keys for a user, sorted by creation date (newest first).
func (d *DB) GetAPIKeysByUser(ctx context.Context, userID string) ([]models.APIKey, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, user_id, name, key_hash, key_prefix, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	return d.scanAPIKeys(rows)
}

// GetAPIKeyByHash looks up an API key by its SHA-256 hash.
// Returns nil if not found or revoked.
func (d *DB) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, user_id, name, key_hash, key_prefix, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE key_hash = ? AND revoked_at IS NULL
	`, keyHash)

	return d.scanAPIKey(row)
}

// RevokeAPIKey marks an API key as revoked.
// Only the key owner can revoke.
func (d *DB) RevokeAPIKey(ctx context.Context, keyID, userID string) error {
	result, err := d.ExecContext(ctx, `
		UPDATE api_keys
		SET revoked_at = datetime('now')
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL
	`, keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check revoke result: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key.
func (d *DB) UpdateAPIKeyLastUsed(ctx context.Context, keyID string) error {
	_, err := d.ExecContext(ctx, `
		UPDATE api_keys
		SET last_used_at = datetime('now')
		WHERE id = ?
	`, keyID)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}

	return nil
}

// scanAPIKey scans a single row into a models.APIKey.
func (d *DB) scanAPIKey(row interface{ Scan(...interface{}) error }) (*models.APIKey, error) {
	var k models.APIKey
	var lastUsedAt, revokedAt sql.NullString
	var createdAtStr string

	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &createdAtStr, &lastUsedAt, &revokedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan api key: %w", err)
	}

	k.CreatedAt, err = parseSQLiteTime(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	if lastUsedAt.Valid {
		t, err := parseSQLiteTime(lastUsedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse last_used_at: %w", err)
		}
		k.LastUsedAt = &t
	}

	if revokedAt.Valid {
		t, err := parseSQLiteTime(revokedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse revoked_at: %w", err)
		}
		k.RevokedAt = &t
	}

	return &k, nil
}

// scanAPIKeys scans rows into a slice of models.APIKey.
func (d *DB) scanAPIKeys(rows *sql.Rows) ([]models.APIKey, error) {
	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		var lastUsedAt, revokedAt sql.NullString
		var createdAtStr string

		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &createdAtStr, &lastUsedAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}

		var pErr error
		k.CreatedAt, pErr = parseSQLiteTime(createdAtStr)
		if pErr != nil {
			return nil, fmt.Errorf("parse created_at: %w", pErr)
		}

		if lastUsedAt.Valid {
			t, err := parseSQLiteTime(lastUsedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse last_used_at: %w", err)
			}
			k.LastUsedAt = &t
		}

		if revokedAt.Valid {
			t, err := parseSQLiteTime(revokedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse revoked_at: %w", err)
			}
			k.RevokedAt = &t
		}

		keys = append(keys, k)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}

	return keys, nil
}
