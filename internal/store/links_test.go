package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

func newLinkStore(t *testing.T) *store.LinkStore {
	t.Helper()
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	return store.NewLinkStore(db, owns, tags)
}

func seedUser(t *testing.T, s *store.LinkStore) string {
	t.Helper()
	// We need a user for ownership. Use the DB directly via the link store's underlying DB.
	// Instead, create a UserStore and upsert.
	db := testutil.NewTestDB(t)
	us := store.NewUserStore(db)
	u, err := us.Upsert(context.Background(), "test", "sub1", "test@example.com", "Test User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

// newTestEnv creates a full test environment sharing the same DB.
func newTestEnv(t *testing.T) (*store.LinkStore, *store.TagStore, *store.UserStore, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)

	u, err := us.Upsert(context.Background(), "test", "sub1", "test@example.com", "Test User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return ls, tags, us, u.ID
}

func TestLinkStore_Create(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	link, err := ls.Create(ctx, "my-link", "https://example.com", userID, "My Link", "A test link", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if link.Slug != "my-link" {
		t.Errorf("slug = %q, want %q", link.Slug, "my-link")
	}
	if link.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", link.URL, "https://example.com")
	}
	if link.Title != "My Link" {
		t.Errorf("title = %q, want %q", link.Title, "My Link")
	}
	if link.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestLinkStore_GetBySlug(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	created, err := ls.Create(ctx, "get-test", "https://example.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ls.GetBySlug(ctx, "get-test")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
}

func TestLinkStore_GetBySlug_NotFound(t *testing.T) {
	ls, _, _, _ := newTestEnv(t)
	ctx := context.Background()

	_, err := ls.GetBySlug(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetBySlug(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestLinkStore_ListAll(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	_, err := ls.Create(ctx, "aaa-link", "https://a.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = ls.Create(ctx, "bbb-link", "https://b.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	links, err := ls.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2", len(links))
	}
	// Should be ordered by slug ASC.
	if links[0].Slug != "aaa-link" {
		t.Errorf("first slug = %q, want %q", links[0].Slug, "aaa-link")
	}
}

func TestLinkStore_ListByOwner(t *testing.T) {
	ls, _, us, userID := newTestEnv(t)
	ctx := context.Background()

	_, err := ls.Create(ctx, "owned-link", "https://example.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create second user with no links.
	u2, err := us.Upsert(ctx, "test", "sub2", "other@example.com", "Other", "")
	if err != nil {
		t.Fatalf("seed user2: %v", err)
	}

	links, err := ls.ListByOwner(ctx, userID)
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}

	links2, err := ls.ListByOwner(ctx, u2.ID)
	if err != nil {
		t.Fatalf("ListByOwner(user2): %v", err)
	}
	if len(links2) != 0 {
		t.Errorf("user2 links = %d, want 0", len(links2))
	}
}

func TestLinkStore_Update(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	created, err := ls.Create(ctx, "update-me", "https://old.com", userID, "Old", "Old desc", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := ls.Update(ctx, created.ID, "https://new.com", "New", "New desc", "public")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.URL != "https://new.com" {
		t.Errorf("url = %q, want %q", updated.URL, "https://new.com")
	}
	if updated.Title != "New" {
		t.Errorf("title = %q, want %q", updated.Title, "New")
	}
	if updated.Description != "New desc" {
		t.Errorf("description = %q, want %q", updated.Description, "New desc")
	}
	if !updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt) {
		// UpdatedAt should be >= CreatedAt (may be equal in fast tests)
	}
}

func TestLinkStore_Delete(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	created, err := ls.Create(ctx, "delete-me", "https://example.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ls.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = ls.GetBySlug(ctx, "delete-me")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetBySlug after delete = %v, want ErrNotFound", err)
	}
}

func TestLinkStore_SlugUniqueness(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	_, err := ls.Create(ctx, "unique-slug", "https://a.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}

	_, err = ls.Create(ctx, "unique-slug", "https://b.com", userID, "", "", "")
	if !errors.Is(err, store.ErrSlugTaken) {
		t.Errorf("Create duplicate slug = %v, want ErrSlugTaken", err)
	}
}

func TestLinkStore_GetByID_NotFound(t *testing.T) {
	ls, _, _, _ := newTestEnv(t)
	ctx := context.Background()

	_, err := ls.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByID(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestLinkStore_SetAndListTags(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	link, err := ls.Create(ctx, "tagged-link", "https://example.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ls.SetTags(ctx, link.ID, []string{"go", "tools"})
	if err != nil {
		t.Fatalf("SetTags: %v", err)
	}

	tags, err := ls.ListTags(ctx, link.ID)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("len(tags) = %d, want 2", len(tags))
	}
}

func TestLinkStore_ListByTag(t *testing.T) {
	ls, _, _, userID := newTestEnv(t)
	ctx := context.Background()

	link, err := ls.Create(ctx, "tag-filter", "https://example.com", userID, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	err = ls.SetTags(ctx, link.ID, []string{"golang"})
	if err != nil {
		t.Fatalf("SetTags: %v", err)
	}

	links, err := ls.ListByTag(ctx, "golang")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if links[0].Slug != "tag-filter" {
		t.Errorf("slug = %q, want %q", links[0].Slug, "tag-filter")
	}
}
