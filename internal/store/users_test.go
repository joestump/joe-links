package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

func TestDeriveDisplayNameSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "Alice Smith", "alice-smith"},
		{"apostrophe and suffix", "Joe O'Brien III", "joe-obrien-iii"},
		{"already lowercase", "alice", "alice"},
		{"leading trailing spaces", "  Bob  ", "bob"},
		{"multiple spaces", "Jane   Doe", "jane-doe"},
		{"special characters", "Test!@#$%User", "testuser"},
		{"consecutive hyphens", "a--b---c", "a-b-c"},
		{"leading trailing hyphens", "-test-", "test"},
		{"empty string", "", ""},
		{"all special chars", "!@#$%^&*()", ""},
		{"unicode letters", "Caf\u00e9 Owner", "caf-owner"},
		{"mixed case", "JoHn DoE", "john-doe"},
		{"dots and commas", "Dr. Jane Smith, PhD", "dr-jane-smith-phd"},
		{"numbers preserved", "User 42", "user-42"},
		{"tabs and newlines", "Tab\tNew\nLine", "tab-new-line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.DeriveDisplayNameSlug(tt.input)
			if got != tt.expected {
				t.Errorf("DeriveDisplayNameSlug(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func newUserStore(t *testing.T) *store.UserStore {
	t.Helper()
	db := testutil.NewTestDB(t)
	return store.NewUserStore(db)
}

func TestGetByDisplayNameSlug(t *testing.T) {
	us := newUserStore(t)
	ctx := context.Background()

	// Create a user via Upsert (which derives and persists the slug).
	u, err := us.Upsert(ctx, "test", "sub1", "alice@example.com", "Alice Smith", "")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Lookup by slug should return the same user.
	found, err := us.GetByDisplayNameSlug(ctx, "alice-smith")
	if err != nil {
		t.Fatalf("GetByDisplayNameSlug: %v", err)
	}
	if found.ID != u.ID {
		t.Errorf("expected user ID %s, got %s", u.ID, found.ID)
	}
	if found.DisplayNameSlug != "alice-smith" {
		t.Errorf("expected slug %q, got %q", "alice-smith", found.DisplayNameSlug)
	}

	// Non-existent slug should return error.
	_, err = us.GetByDisplayNameSlug(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent slug, got nil")
	}
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestResolveUniqueSlug_Duplicates(t *testing.T) {
	us := newUserStore(t)
	ctx := context.Background()

	// Create two users with the same display name.
	u1, err := us.Upsert(ctx, "test", "sub1", "alice1@example.com", "Alice Smith", "")
	if err != nil {
		t.Fatalf("upsert user 1: %v", err)
	}
	u2, err := us.Upsert(ctx, "test", "sub2", "alice2@example.com", "Alice Smith", "")
	if err != nil {
		t.Fatalf("upsert user 2: %v", err)
	}

	// First user should get the base slug, second should get a suffix.
	if u1.DisplayNameSlug != "alice-smith" {
		t.Errorf("user 1 slug = %q, want %q", u1.DisplayNameSlug, "alice-smith")
	}
	if u2.DisplayNameSlug != "alice-smith-2" {
		t.Errorf("user 2 slug = %q, want %q", u2.DisplayNameSlug, "alice-smith-2")
	}

	// A third duplicate should get -3.
	u3, err := us.Upsert(ctx, "test", "sub3", "alice3@example.com", "Alice Smith", "")
	if err != nil {
		t.Fatalf("upsert user 3: %v", err)
	}
	if u3.DisplayNameSlug != "alice-smith-3" {
		t.Errorf("user 3 slug = %q, want %q", u3.DisplayNameSlug, "alice-smith-3")
	}
}

func TestUpsert_UpdatesSlugOnNameChange(t *testing.T) {
	us := newUserStore(t)
	ctx := context.Background()

	// Create a user.
	u, err := us.Upsert(ctx, "test", "sub1", "bob@example.com", "Bob Jones", "")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if u.DisplayNameSlug != "bob-jones" {
		t.Errorf("initial slug = %q, want %q", u.DisplayNameSlug, "bob-jones")
	}

	// Re-login with a changed display name should update the slug.
	u2, err := us.Upsert(ctx, "test", "sub1", "bob@example.com", "Robert Jones", "")
	if err != nil {
		t.Fatalf("upsert with new name: %v", err)
	}
	if u2.DisplayNameSlug != "robert-jones" {
		t.Errorf("updated slug = %q, want %q", u2.DisplayNameSlug, "robert-jones")
	}
}

func TestUpsert_SpecialCharacterSlug(t *testing.T) {
	us := newUserStore(t)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub1", "joe@example.com", "Joe O'Brien III", "")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if u.DisplayNameSlug != "joe-obrien-iii" {
		t.Errorf("slug = %q, want %q", u.DisplayNameSlug, "joe-obrien-iii")
	}
}
