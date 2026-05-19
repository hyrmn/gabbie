package models

import (
	"testing"
	"time"
)

func TestItem_IsOverdue(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name string
		item Item
		want bool
	}{
		{"todo with past due", Item{Status: StatusTodo, DueDate: &past}, true},
		{"todo with future due", Item{Status: StatusTodo, DueDate: &future}, false},
		{"done with past due", Item{Status: StatusDone, DueDate: &past}, false},
		{"no due date", Item{Status: StatusTodo, DueDate: nil}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.IsOverdue(); got != tt.want {
				t.Errorf("IsOverdue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItem_HasTag(t *testing.T) {
	item := Item{Tags: []string{"urgent", "Backend", "API"}}

	tests := []struct {
		tag  string
		want bool
	}{
		{"urgent", true},
		{"backend", true}, // case insensitive
		{"api", true},
		{"frontend", false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			if got := item.HasTag(tt.tag); got != tt.want {
				t.Errorf("HasTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestItem_MarshalUnmarshalTags(t *testing.T) {
	item := Item{Tags: []string{"bug", "critical"}}
	data, err := item.MarshalTags()
	if err != nil {
		t.Fatal(err)
	}

	var decoded Item
	if err := decoded.UnmarshalTags(data); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Tags) != 2 || decoded.Tags[0] != "bug" || decoded.Tags[1] != "critical" {
		t.Errorf("decoded tags = %v, want [bug critical]", decoded.Tags)
	}

	// Test empty
	var empty Item
	if err := empty.UnmarshalTags(""); err != nil {
		t.Fatal(err)
	}
	if len(empty.Tags) != 0 {
		t.Errorf("empty tags = %v, want []", empty.Tags)
	}
}

func TestAPIKey_IsActive(t *testing.T) {
	active := APIKey{RevokedAt: nil}
	if !active.IsActive() {
		t.Error("expected active key to be active")
	}

	now := time.Now()
	revoked := APIKey{RevokedAt: &now}
	if revoked.IsActive() {
		t.Error("expected revoked key to be inactive")
	}
}

func TestAPIKey_MaskedKey(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"ctx7sk-98", "ctx7...98"},
		{"abc", "abc..."},
		{"test", "test..."},
	}

	for _, tt := range tests {
		k := APIKey{KeyPrefix: tt.prefix}
		if got := k.MaskedKey(); got != tt.want {
			t.Errorf("MaskedKey(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}
