// Package db provides SQLite database connection and query helpers.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hyrmn/todotracker/internal/models"
)

// ItemFilter holds filtering and sorting options.
type ItemFilter struct {
	Status     string
	AssigneeID *string
	Priority   *int
	Tag        string
	SortBy     string // "due_date", "priority", "created_at", "sort_order"
	SortDir    string // "asc", "desc"
}

// ItemUpdates holds partial update fields for UpdateItemPartial.
type ItemUpdates struct {
	Title       *string
	Description *string
	Status      *string
	AssigneeID  *string
	Priority    *int
	DueDate     *string
	Tags        []string
}

// UpdateItemPartial applies partial updates to an item using ItemUpdates.
func (d *DB) UpdateItemPartial(ctx context.Context, itemID string, updates ItemUpdates) error {
	var sets []string
	var args []any

	if updates.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *updates.Title)
	}
	if updates.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *updates.Description)
	}
	if updates.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *updates.Status)
	}
	if updates.AssigneeID != nil {
		sets = append(sets, "assignee_id = ?")
		args = append(args, *updates.AssigneeID)
	}
	if updates.Priority != nil {
		sets = append(sets, "priority = ?")
		args = append(args, *updates.Priority)
	}
	if updates.DueDate != nil {
		sets = append(sets, "due_date = ?")
		args = append(args, *updates.DueDate)
	}
	if updates.Tags != nil {
		tagsJSON, err := marshalTagsJSON(updates.Tags)
		if err != nil {
			return fmt.Errorf("marshal tags: %w", err)
		}
		sets = append(sets, "tags = ?")
		args = append(args, tagsJSON)
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, "updated_at = datetime('now')")
	args = append(args, itemID)

	query := fmt.Sprintf("UPDATE items SET %s WHERE id = ?", strings.Join(sets, ", "))

	result, err := d.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
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

// ToggleItemStatus cycles the status: todo → in_progress → done → todo.
func (d *DB) ToggleItemStatus(ctx context.Context, itemID string) (string, error) {
	var currentStatus string
	err := d.QueryRowContext(ctx, `SELECT status FROM items WHERE id = ?`, itemID).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("item not found")
		}
		return "", fmt.Errorf("get item status: %w", err)
	}

	var nextStatus string
	switch currentStatus {
	case string(models.StatusTodo):
		nextStatus = string(models.StatusInProgress)
	case string(models.StatusInProgress):
		nextStatus = string(models.StatusDone)
	case string(models.StatusDone):
		nextStatus = string(models.StatusTodo)
	default:
		nextStatus = string(models.StatusTodo)
	}

	_, err = d.ExecContext(ctx, `UPDATE items SET status = ?, updated_at = datetime('now') WHERE id = ?`, nextStatus, itemID)
	if err != nil {
		return "", fmt.Errorf("toggle item status: %w", err)
	}

	return nextStatus, nil
}

// ItemDetail extends models.Item with display-friendly fields.
type ItemDetail struct {
	models.Item
	AssignedEmail       string
	AssignedDisplayName string
	IsOverdue           bool
}

// GetItemsWithDetails returns items with assignee info and overdue flags.
func (d *DB) GetItemsWithDetails(ctx context.Context, listID string, filter ItemFilter) ([]ItemDetail, error) {
	baseQuery := `
		SELECT i.id, i.list_id, i.title, i.description, i.status, i.assignee_id,
		       i.priority, i.due_date, i.tags, i.sort_order, i.created_at, i.updated_at,
		       COALESCE(u.email, ''), COALESCE(u.display_name, '')
		FROM items i
		LEFT JOIN users u ON u.id = i.assignee_id
		WHERE i.list_id = ?
	`
	args := []any{listID}

	var whereClauses []string
	if filter.Status != "" {
		whereClauses = append(whereClauses, "i.status = ?")
		args = append(args, filter.Status)
	}
	if filter.AssigneeID != nil {
		whereClauses = append(whereClauses, "i.assignee_id = ?")
		args = append(args, *filter.AssigneeID)
	}
	if filter.Priority != nil {
		whereClauses = append(whereClauses, "i.priority = ?")
		args = append(args, *filter.Priority)
	}
	if filter.Tag != "" {
		whereClauses = append(whereClauses, "i.tags LIKE '%' || ? || '%'")
		args = append(args, filter.Tag)
	}
	if len(whereClauses) > 0 {
		baseQuery += " AND " + strings.Join(whereClauses, " AND ")
	}

	sortBy := "i.created_at"
	sortDir := "asc"
	switch filter.SortBy {
	case "due_date":
		sortBy = "i.due_date"
	case "priority":
		sortBy = "i.priority"
	case "sort_order":
		sortBy = "i.sort_order"
	}
	if strings.ToLower(filter.SortDir) == "desc" {
		sortDir = "desc"
	}
	baseQuery += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortDir)

	rows, err := d.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query items with details: %w", err)
	}
	defer rows.Close()

	var items []ItemDetail
	for rows.Next() {
		var detail ItemDetail
		var assigneeID sql.NullString
		var dueDate sql.NullString
		var tags string
		var createdAtStr, updatedAtStr string

		if err := rows.Scan(
			&detail.ID, &detail.ListID, &detail.Title, &detail.Description, &detail.Status,
			&assigneeID, &detail.Priority, &dueDate, &tags, &detail.SortOrder,
			&createdAtStr, &updatedAtStr, &detail.AssignedEmail, &detail.AssignedDisplayName,
		); err != nil {
			return nil, fmt.Errorf("scan item detail: %w", err)
		}

		if assigneeID.Valid {
			detail.AssigneeID = &assigneeID.String
		}
		if dueDate.Valid {
			detail.DueDate = &dueDate.String
		}

		if err := detail.UnmarshalTags(tags); err != nil {
			return nil, fmt.Errorf("parse tags: %w", err)
		}

		detail.CreatedAt, err = parseSQLiteTime(createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		detail.UpdatedAt, err = parseSQLiteTime(updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}

		detail.IsOverdue = detail.Item.IsOverdue()
		items = append(items, detail)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}

	return items, nil
}

// GetItemsByList returns all items for a list that the user has access to.
// Supports filtering and sorting via ItemFilter.
func (d *DB) GetItemsByList(ctx context.Context, listID, userID string, filter ItemFilter) ([]models.Item, error) {
	baseQuery := `
		SELECT i.id, i.list_id, i.title, i.description, i.status, i.assignee_id, i.priority,
		       i.due_date, i.tags, i.sort_order, i.created_at, i.updated_at
		FROM items i
		JOIN list_collaborators lc ON lc.list_id = i.list_id
		WHERE i.list_id = ? AND lc.user_id = ?
	`
	args := []any{listID, userID}

	var whereClauses []string
	if filter.Status != "" {
		whereClauses = append(whereClauses, "i.status = ?")
		args = append(args, filter.Status)
	}
	if filter.AssigneeID != nil {
		whereClauses = append(whereClauses, "i.assignee_id = ?")
		args = append(args, *filter.AssigneeID)
	}
	if filter.Priority != nil {
		whereClauses = append(whereClauses, "i.priority = ?")
		args = append(args, *filter.Priority)
	}
	if filter.Tag != "" {
		whereClauses = append(whereClauses, "i.tags LIKE '%' || ? || '%'")
		args = append(args, filter.Tag)
	}
	if len(whereClauses) > 0 {
		baseQuery += " AND " + strings.Join(whereClauses, " AND ")
	}

	sortBy := "i.created_at"
	sortDir := "asc"
	switch filter.SortBy {
	case "due_date":
		sortBy = "i.due_date"
	case "priority":
		sortBy = "i.priority"
	case "sort_order":
		sortBy = "i.sort_order"
	}
	if strings.ToLower(filter.SortDir) == "desc" {
		sortDir = "desc"
	}
	baseQuery += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortDir)

	rows, err := d.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query items by list: %w", err)
	}
	defer rows.Close()

	return d.scanItems(rows)
}

// GetItemsByListSimple returns all items for a list without access control.
// Use for API calls where access is checked at handler level.
func (d *DB) GetItemsByListSimple(ctx context.Context, listID string) ([]models.Item, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, list_id, title, description, status, assignee_id, priority,
		       due_date, tags, sort_order, created_at, updated_at
		FROM items
		WHERE list_id = ?
		ORDER BY sort_order ASC, created_at ASC
	`, listID)
	if err != nil {
		return nil, fmt.Errorf("query items by list: %w", err)
	}
	defer rows.Close()

	return d.scanItems(rows)
}

// GetItem returns a single item if the user has access to its list.
func (d *DB) GetItem(ctx context.Context, itemID, userID string) (*models.Item, error) {
	row := d.QueryRowContext(ctx, `
		SELECT i.id, i.list_id, i.title, i.description, i.status, i.assignee_id,
		       i.priority, i.due_date, i.tags, i.sort_order, i.created_at, i.updated_at
		FROM items i
		JOIN list_collaborators lc ON lc.list_id = i.list_id
		WHERE i.id = ? AND lc.user_id = ?
	`, itemID, userID)

	var item models.Item
	var assigneeID sql.NullString
	var dueDate sql.NullString
	var tags string
	var createdAtStr, updatedAtStr string

	err := row.Scan(&item.ID, &item.ListID, &item.Title, &item.Description, &item.Status,
		&assigneeID, &item.Priority, &dueDate, &tags, &item.SortOrder, &createdAtStr, &updatedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get item: %w", err)
	}

	if assigneeID.Valid {
		item.AssigneeID = &assigneeID.String
	}
	if dueDate.Valid {
		item.DueDate = &dueDate.String
	}

	if err := item.UnmarshalTags(tags); err != nil {
		return nil, fmt.Errorf("parse tags: %w", err)
	}

	item.CreatedAt, err = parseSQLiteTime(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	item.UpdatedAt, err = parseSQLiteTime(updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &item, nil
}

// GetItemSimple returns a single item by ID without access control.
func (d *DB) GetItemSimple(ctx context.Context, itemID string) (*models.Item, error) {
	row := d.QueryRowContext(ctx, `
		SELECT i.id, i.list_id, i.title, i.description, i.status, i.assignee_id,
		       i.priority, i.due_date, i.tags, i.sort_order, i.created_at, i.updated_at
		FROM items i
		WHERE i.id = ?
	`, itemID)

	var item models.Item
	var assigneeID sql.NullString
	var dueDate sql.NullString
	var tags string
	var createdAtStr, updatedAtStr string

	err := row.Scan(&item.ID, &item.ListID, &item.Title, &item.Description, &item.Status,
		&assigneeID, &item.Priority, &dueDate, &tags, &item.SortOrder, &createdAtStr, &updatedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get item: %w", err)
	}

	if assigneeID.Valid {
		item.AssigneeID = &assigneeID.String
	}
	if dueDate.Valid {
		item.DueDate = &dueDate.String
	}

	if err := item.UnmarshalTags(tags); err != nil {
		return nil, fmt.Errorf("parse tags: %w", err)
	}

	item.CreatedAt, err = parseSQLiteTime(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	item.UpdatedAt, err = parseSQLiteTime(updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &item, nil
}

// CreateItem inserts a new item into a list.
func (d *DB) CreateItem(ctx context.Context, listID, title, description string,
	status models.ItemStatus, assigneeID *string, priority int,
	dueDate *string, tags []string, sortOrder int) (*models.Item, error) {

	id := uuid.New().String()

	tagsJSON, err := marshalTagsJSON(tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}

	_, err = d.ExecContext(ctx, `
		INSERT INTO items (id, list_id, title, description, status, assignee_id, priority,
		                   due_date, tags, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, listID, title, description, status, nullString(assigneeID), priority,
		nullString(dueDate), tagsJSON, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}

	return d.GetItem(ctx, id, "") // no user access check — only called right after create
}

// CreateItemSimple creates an item with default values (todo, medium priority).
func (d *DB) CreateItemSimple(ctx context.Context, listID, title string) (*models.Item, error) {
	return d.CreateItem(ctx, listID, title, "", models.StatusTodo, nil, models.PriorityMedium, nil, nil, 0)
}


// UpdateItem updates an item's fields. The user must have access to the list.
// Fields that are empty strings or nil are not updated (partial update).
func (d *DB) UpdateItem(ctx context.Context, itemID, userID string,
	title, description string, status string, assigneeID *string,
	priority int, dueDate *string, tags []string, sortOrder int) (*models.Item, error) {

	// First check user has access
	var listID string
	err := d.QueryRowContext(ctx, `
		SELECT i.list_id
		FROM items i
		JOIN list_collaborators lc ON lc.list_id = i.list_id
		WHERE i.id = ? AND lc.user_id = ?
	`, itemID, userID).Scan(&listID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("check item access: %w", err)
	}

	// Build dynamic update query with only provided fields
	updates := []string{"updated_at = datetime('now')"}
	args := []interface{}{}

	if title != "" {
		updates = append(updates, "title = ?")
		args = append(args, title)
	}
	if description != "" {
		updates = append(updates, "description = ?")
		args = append(args, description)
	}
	if status != "" {
		updates = append(updates, "status = ?")
		args = append(args, models.ItemStatus(status))
	}
	if assigneeID != nil {
		updates = append(updates, "assignee_id = ?")
		args = append(args, nullString(assigneeID))
	}
	if priority >= 0 {
		updates = append(updates, "priority = ?")
		args = append(args, priority)
	}
	if dueDate != nil {
		updates = append(updates, "due_date = ?")
		args = append(args, nullString(dueDate))
	}
	if len(tags) > 0 {
		tagsJSON, err := marshalTagsJSON(tags)
		if err != nil {
			return nil, fmt.Errorf("marshal tags: %w", err)
		}
		updates = append(updates, "tags = ?")
		args = append(args, tagsJSON)
	}

	args = append(args, itemID)
	query := fmt.Sprintf("UPDATE items SET %s WHERE id = ?", strings.Join(updates, ", "))

	result, err := d.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update item: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check update result: %w", err)
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}

	return d.GetItem(ctx, itemID, userID)
}

// DeleteItem removes an item from a list. User must have access to the list.
func (d *DB) DeleteItem(ctx context.Context, itemID, userID string) error {
	result, err := d.ExecContext(ctx, `
		DELETE FROM items
		WHERE id = ? AND list_id IN (
			SELECT list_id FROM list_collaborators WHERE user_id = ?
		)
	`, itemID, userID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
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

// DeleteItemSimple removes an item by ID without access control.
func (d *DB) DeleteItemSimple(ctx context.Context, itemID string) error {
	result, err := d.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, itemID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
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

// scanItems scans rows into a slice of models.Item.
func (d *DB) scanItems(rows *sql.Rows) ([]models.Item, error) {
	var items []models.Item
	for rows.Next() {
		var item models.Item
		var assigneeID sql.NullString
		var dueDate sql.NullString
		var tags string
		var createdAtStr, updatedAtStr string

		if err := rows.Scan(&item.ID, &item.ListID, &item.Title, &item.Description, &item.Status,
			&assigneeID, &item.Priority, &dueDate, &tags, &item.SortOrder, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}

		if assigneeID.Valid {
			item.AssigneeID = &assigneeID.String
		}
		if dueDate.Valid {
			item.DueDate = &dueDate.String
		}

		if err := item.UnmarshalTags(tags); err != nil {
			return nil, fmt.Errorf("parse tags: %w", err)
		}

		var pErr error
		item.CreatedAt, pErr = parseSQLiteTime(createdAtStr)
		if pErr != nil {
			return nil, fmt.Errorf("parse created_at: %w", pErr)
		}
		item.UpdatedAt, pErr = parseSQLiteTime(updatedAtStr)
		if pErr != nil {
			return nil, fmt.Errorf("parse updated_at: %w", pErr)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}

	return items, nil
}

// nullString converts a *string to a value suitable for SQL (nil or the string).
func nullString(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

// marshalTagsJSON marshals a string slice to JSON.
func marshalTagsJSON(tags []string) (string, error) {
	if tags == nil {
		return "[]", nil
	}
	return models.Item{Tags: tags}.MarshalTags()
}
