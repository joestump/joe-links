package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

func newTagTestEnv(t *testing.T) (*store.TagStore, *store.LinkStore, *store.UserStore) {
	t.Helper()
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	return tags, ls, us
}

func TestTagStore_Upsert_Create(t *testing.T) {
	ts, _, _ := newTagTestEnv(t)
	ctx := context.Background()

	tag, err := ts.Upsert(ctx, "Golang")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if tag.Name != "Golang" {
		t.Errorf("name = %q, want %q", tag.Name, "Golang")
	}
	if tag.Slug != "golang" {
		t.Errorf("slug = %q, want %q", tag.Slug, "golang")
	}
	if tag.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestTagStore_Upsert_Idempotent(t *testing.T) {
	ts, _, _ := newTagTestEnv(t)
	ctx := context.Background()

	tag1, err := ts.Upsert(ctx, "Golang")
	if err != nil {
		t.Fatalf("Upsert first: %v", err)
	}

	tag2, err := ts.Upsert(ctx, "golang")
	if err != nil {
		t.Fatalf("Upsert second: %v", err)
	}

	if tag1.ID != tag2.ID {
		t.Errorf("expected same ID, got %q and %q", tag1.ID, tag2.ID)
	}
}

func TestTagStore_GetBySlug(t *testing.T) {
	ts, _, _ := newTagTestEnv(t)
	ctx := context.Background()

	created, err := ts.Upsert(ctx, "My Tag")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := ts.GetBySlug(ctx, "my-tag")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
}

func TestTagStore_GetBySlug_NotFound(t *testing.T) {
	ts, _, _ := newTagTestEnv(t)
	ctx := context.Background()

	_, err := ts.GetBySlug(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetBySlug(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestTagStore_ListAll(t *testing.T) {
	ts, _, _ := newTagTestEnv(t)
	ctx := context.Background()

	_, err := ts.Upsert(ctx, "Beta")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	_, err = ts.Upsert(ctx, "Alpha")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	tags, err := ts.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("len = %d, want 2", len(tags))
	}
	// Should be ordered by name ASC.
	if tags[0].Name != "Alpha" {
		t.Errorf("first tag = %q, want %q", tags[0].Name, "Alpha")
	}
}

func TestTagStore_ListWithCounts(t *testing.T) {
	ts, ls, us := newTagTestEnv(t)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub1", "test@example.com", "Test", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Create a link with a tag.
	link, err := ls.Create(ctx, "counted", "https://example.com", u.ID, "", "", "")
	if err != nil {
		t.Fatalf("Create link: %v", err)
	}
	err = ls.SetTags(ctx, link.ID, []string{"popular"})
	if err != nil {
		t.Fatalf("SetTags: %v", err)
	}

	// Create a tag with no links.
	_, err = ts.Upsert(ctx, "orphan")
	if err != nil {
		t.Fatalf("Upsert orphan: %v", err)
	}

	tags, err := ts.ListWithCounts(ctx)
	if err != nil {
		t.Fatalf("ListWithCounts: %v", err)
	}

	// Only "popular" should appear (orphan has 0 links).
	if len(tags) != 1 {
		t.Fatalf("len = %d, want 1", len(tags))
	}
	if tags[0].Slug != "popular" {
		t.Errorf("slug = %q, want %q", tags[0].Slug, "popular")
	}
	if tags[0].Count != 1 {
		t.Errorf("count = %d, want 1", tags[0].Count)
	}
}
