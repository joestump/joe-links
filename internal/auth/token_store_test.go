package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

func newTokenTestEnv(t *testing.T) (*auth.SQLTokenStore, *store.UserStore, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	ts := auth.NewSQLTokenStore(db)
	us := store.NewUserStore(db)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub1", "test@example.com", "Test User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return ts, us, u.ID
}

func TestGenerateToken(t *testing.T) {
	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if len(plaintext) < 10 {
		t.Errorf("plaintext too short: %q", plaintext)
	}
	if plaintext[:3] != "jl_" {
		t.Errorf("plaintext prefix = %q, want %q", plaintext[:3], "jl_")
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}

	// HashToken should produce the same hash.
	if got := auth.HashToken(plaintext); got != hash {
		t.Errorf("HashToken = %q, want %q", got, hash)
	}
}

func TestTokenStore_CreateAndGetByHash(t *testing.T) {
	ts, _, userID := newTokenTestEnv(t)
	ctx := context.Background()

	_, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	rec, err := ts.Create(ctx, userID, "test-token", hash, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.UserID != userID {
		t.Errorf("UserID = %q, want %q", rec.UserID, userID)
	}
	if rec.Name != "test-token" {
		t.Errorf("Name = %q, want %q", rec.Name, "test-token")
	}

	got, err := ts.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.ID != rec.ID {
		t.Errorf("ID = %q, want %q", got.ID, rec.ID)
	}
}

func TestTokenStore_GetByHash_NotFound(t *testing.T) {
	ts, _, _ := newTokenTestEnv(t)
	ctx := context.Background()

	_, err := ts.GetByHash(ctx, "nonexistent-hash")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByHash(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestTokenStore_Revoke(t *testing.T) {
	ts, _, userID := newTokenTestEnv(t)
	ctx := context.Background()

	_, hash, _ := auth.GenerateToken()
	rec, err := ts.Create(ctx, userID, "revoke-me", hash, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ts.Revoke(ctx, rec.ID, userID)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, err := ts.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash after revoke: %v", err)
	}
	if !got.RevokedAt.Valid {
		t.Error("expected RevokedAt to be set after revoke")
	}
}

func TestTokenStore_Revoke_NotFound(t *testing.T) {
	ts, _, userID := newTokenTestEnv(t)
	ctx := context.Background()

	err := ts.Revoke(ctx, "nonexistent-id", userID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Revoke(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestTokenStore_ExpiredToken(t *testing.T) {
	ts, _, userID := newTokenTestEnv(t)
	ctx := context.Background()

	_, hash, _ := auth.GenerateToken()
	expired := time.Now().Add(-1 * time.Hour)
	rec, err := ts.Create(ctx, userID, "expired-token", hash, &expired)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ts.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	// The store returns the record; it's up to the middleware to check expiry.
	if got.ID != rec.ID {
		t.Errorf("ID = %q, want %q", got.ID, rec.ID)
	}
	if !got.ExpiresAt.Valid {
		t.Error("expected ExpiresAt to be set")
	}
	if !got.ExpiresAt.Time.Before(time.Now()) {
		t.Error("expected ExpiresAt to be in the past")
	}
}

func TestTokenStore_ListByUser(t *testing.T) {
	ts, _, userID := newTokenTestEnv(t)
	ctx := context.Background()

	_, hash1, _ := auth.GenerateToken()
	_, err := ts.Create(ctx, userID, "token-1", hash1, nil)
	if err != nil {
		t.Fatalf("Create token-1: %v", err)
	}

	_, hash2, _ := auth.GenerateToken()
	_, err = ts.Create(ctx, userID, "token-2", hash2, nil)
	if err != nil {
		t.Fatalf("Create token-2: %v", err)
	}

	records, err := ts.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len = %d, want 2", len(records))
	}
}

func TestTokenStore_UpdateLastUsed(t *testing.T) {
	ts, _, userID := newTokenTestEnv(t)
	ctx := context.Background()

	_, hash, _ := auth.GenerateToken()
	rec, err := ts.Create(ctx, userID, "track-usage", hash, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if rec.LastUsedAt.Valid {
		t.Error("expected LastUsedAt to be null initially")
	}

	err = ts.UpdateLastUsed(ctx, rec.ID)
	if err != nil {
		t.Fatalf("UpdateLastUsed: %v", err)
	}

	got, err := ts.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if !got.LastUsedAt.Valid {
		t.Error("expected LastUsedAt to be set after update")
	}
}
