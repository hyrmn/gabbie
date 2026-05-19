// Package db provides SQLite database connection and query helpers.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/hyrmn/todotracker/internal/models"
)

// ListQueries contains all list-related database operations.

// CreateList inserts a new list into the database.
func (d *DB) CreateList(ctx context.Context, name, description, ownerID string) (*models.List, error) {
	id := uuid.New().String()

	_, err := d.ExecContext(ctx, `
		INSERT INTO lists (id, name, description, owner_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, name, description, ownerID)
	if err != nil {
		return nil, fmt.Errorf("create list: %w", err)
	}

	// Fetch the created list
	return d.GetList(ctx, id, ownerID)
}

// GetListsByUser returns all lists where the user is owner or collaborator.
func (d *DB) GetListsByUser(ctx context.Context, userID string) ([]models.List, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT DISTINCT l.id, l.name, l.description, l.owner_id, l.created_at, l.updated_at
		FROM lists l
		JOIN list_collaborators lc ON lc.list_id = l.id
		WHERE lc.user_id = ?
		ORDER BY l.updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query lists by user: %w", err)
	}
	defer rows.Close()

	return d.scanLists(rows)
}

// GetList returns a single list if the user has access (owner or collaborator).
func (d *DB) GetList(ctx context.Context, id, userID string) (*models.List, error) {
	row := d.QueryRowContext(ctx, `
		SELECT l.id, l.name, l.description, l.owner_id, l.created_at, l.updated_at
		FROM lists l
		JOIN list_collaborators lc ON lc.list_id = l.id
		WHERE l.id = ? AND lc.user_id = ?
	`, id, userID)

	var list models.List
	var createdAtStr, updatedAtStr string
	err := row.Scan(&list.ID, &list.Name, &list.Description, &list.OwnerID, &createdAtStr, &updatedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found or no access
		}
		return nil, fmt.Errorf("get list: %w", err)
	}

	list.CreatedAt, err = parseSQLiteTime(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	list.UpdatedAt, err = parseSQLiteTime(updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &list, nil
}

// UpdateList updates the name and description of a list.
func (d *DB) UpdateList(ctx context.Context, id, name, description string) error {
	result, err := d.ExecContext(ctx, `
		UPDATE lists
		SET name = ?, description = ?, updated_at = datetime('now')
		WHERE id = ?
	`, name, description, id)
	if err != nil {
		return fmt.Errorf("update list: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteList removes a list. Only the owner can delete.
func (d *DB) DeleteList(ctx context.Context, id, ownerID string) error {
	result, err := d.ExecContext(ctx, `
		DELETE FROM lists
		WHERE id = ? AND owner_id = ?
	`, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete list: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// IsListOwner returns true if the user owns the list.
func (d *DB) IsListOwner(ctx context.Context, listID, userID string) (bool, error) {
	var exists bool
	err := d.QueryRowContext(ctx, `
		SELECT 1 FROM lists WHERE id = ? AND owner_id = ?
	`, listID, userID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check list owner: %w", err)
	}
	return true, nil
}

// GetUserByEmail finds a user by email address.
func (d *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	row := d.QueryRowContext(ctx, `
		SELECT id, email, display_name, created_at, updated_at
		FROM users
		WHERE email = ?
	`, email)

	var user models.User
	var createdAtStr, updatedAtStr string
	err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &createdAtStr, &updatedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	user.CreatedAt, err = parseSQLiteTime(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	user.UpdatedAt, err = parseSQLiteTime(updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &user, nil
}

// CollaboratorDetail extends ListCollaborator with user display info.
type CollaboratorDetail struct {
	models.ListCollaborator
	Email       string
	DisplayName string
}

// GetCollaborators returns all collaborators for a list with user info.
func (d *DB) GetCollaborators(ctx context.Context, listID string) ([]CollaboratorDetail, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT lc.list_id, lc.user_id, lc.role, lc.invited_at, u.email, u.display_name
		FROM list_collaborators lc
		JOIN users u ON u.id = lc.user_id
		WHERE lc.list_id = ?
		ORDER BY lc.role DESC, u.email ASC
	`, listID)
	if err != nil {
		return nil, fmt.Errorf("query collaborators: %w", err)
	}
	defer rows.Close()

	var collabs []CollaboratorDetail
	for rows.Next() {
		var c CollaboratorDetail
		var invitedAtStr string
		if err := rows.Scan(&c.ListID, &c.UserID, &c.Role, &invitedAtStr, &c.Email, &c.DisplayName); err != nil {
			return nil, fmt.Errorf("scan collaborator: %w", err)
		}
		c.InvitedAt, err = parseSQLiteTime(invitedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse invited_at: %w", err)
		}
		collabs = append(collabs, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collaborators: %w", err)
	}

	return collabs, nil
}

// AddCollaborator adds a user as a collaborator to a list.
func (d *DB) AddCollaborator(ctx context.Context, listID, userID, role string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO list_collaborators (list_id, user_id, role, invited_at)
		VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(list_id, user_id) DO UPDATE SET
			role = excluded.role,
			invited_at = datetime('now')
	`, listID, userID, role)
	if err != nil {
		return fmt.Errorf("add collaborator: %w", err)
	}
	return nil
}

// RemoveCollaborator removes a user from a list's collaborators.
// The owner cannot be removed.
func (d *DB) RemoveCollaborator(ctx context.Context, listID, userID string) error {
	// Don't allow removing the owner
	var ownerID string
	err := d.QueryRowContext(ctx, `SELECT owner_id FROM lists WHERE id = ?`, listID).Scan(&ownerID)
	if err != nil {
		return fmt.Errorf("get list owner: %w", err)
	}
	if ownerID == userID {
		return fmt.Errorf("cannot remove the list owner")
	}

	result, err := d.ExecContext(ctx, `
		DELETE FROM list_collaborators
		WHERE list_id = ? AND user_id = ?
	`, listID, userID)
	if err != nil {
		return fmt.Errorf("remove collaborator: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check remove result: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// scanLists scans rows into a slice of models.List.
func (d *DB) scanLists(rows *sql.Rows) ([]models.List, error) {
	var lists []models.List
	for rows.Next() {
		var l models.List
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&l.ID, &l.Name, &l.Description, &l.OwnerID, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan list: %w", err)
		}
		var err error
		l.CreatedAt, err = parseSQLiteTime(createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		l.UpdatedAt, err = parseSQLiteTime(updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}
		lists = append(lists, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lists: %w", err)
	}
	return lists, nil
}
