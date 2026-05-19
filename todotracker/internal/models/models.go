// Package models defines the data types for the todotracker application.
package models

import (
	"encoding/json"
	"strings"
	"time"
)

// User represents an application user linked to a Supabase account.
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// List represents a shared to-do list.
type List struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CollaboratorRole represents a user's role in a list.
type CollaboratorRole string

const (
	RoleOwner  CollaboratorRole = "owner"
	RoleMember CollaboratorRole = "member"
)

// ListCollaborator represents a user's collaboration on a list.
type ListCollaborator struct {
	ListID    string           `json:"list_id"`
	UserID    string           `json:"user_id"`
	Role      CollaboratorRole `json:"role"`
	InvitedAt time.Time        `json:"invited_at"`
}

// ItemStatus represents the status of a to-do item.
type ItemStatus string

const (
	StatusTodo       ItemStatus = "todo"
	StatusInProgress ItemStatus = "in_progress"
	StatusDone       ItemStatus = "done"
)

// Priority levels for to-do items.
const (
	PriorityCritical = 0
	PriorityHigh     = 1
	PriorityMedium   = 2
	PriorityLow      = 3
)

// Item represents a single to-do item in a list.
type Item struct {
	ID          string     `json:"id"`
	ListID      string     `json:"list_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      ItemStatus `json:"status"`
	AssigneeID  *string    `json:"assignee_id"` // nullable Supabase user UUID
	Priority    int        `json:"priority"`    // 0-3
	DueDate     *string    `json:"due_date"`    // nullable ISO 8601 date
	Tags        []string   `json:"tags"`
	SortOrder   int        `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsOverdue returns true if the item has a due date in the past and is not done.
func (i Item) IsOverdue() bool {
	if i.DueDate == nil || i.Status == StatusDone {
		return false
	}
	due, err := time.Parse(time.RFC3339, *i.DueDate)
	if err != nil {
		// Try date-only format
		due, err = time.Parse("2006-01-02", *i.DueDate)
		if err != nil {
			return false
		}
	}
	return time.Now().After(due)
}

// HasTag returns true if the item has the given tag (case-insensitive).
func (i Item) HasTag(tag string) bool {
	tag = strings.ToLower(tag)
	for _, t := range i.Tags {
		if strings.ToLower(t) == tag {
			return true
		}
	}
	return false
}

// MarshalTags returns the tags as a JSON string for database storage.
func (i Item) MarshalTags() (string, error) {
	data, err := json.Marshal(i.Tags)
	if err != nil {
		return "[]", err
	}
	return string(data), nil
}

// UnmarshalTags parses a JSON string into the tags slice.
func (i *Item) UnmarshalTags(data string) error {
	if data == "" {
		i.Tags = []string{}
		return nil
	}
	return json.Unmarshal([]byte(data), &i.Tags)
}

// APIKey represents a user-generated API key.
type APIKey struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"-"`            // never serialize the hash
	KeyPrefix  string     `json:"key_prefix"`   // first 8 chars for display
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// IsActive returns true if the API key has not been revoked.
func (k APIKey) IsActive() bool {
	return k.RevokedAt == nil
}

// MaskedKey returns a display-safe version of the key prefix.
// Example: "ctx7sk-9..." for a key with prefix "ctx7sk-98"
func (k APIKey) MaskedKey() string {
	if len(k.KeyPrefix) <= 4 {
		return k.KeyPrefix + "..."
	}
	return k.KeyPrefix[:4] + "..." + k.KeyPrefix[len(k.KeyPrefix)-2:]
}
