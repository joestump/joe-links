package auth_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

// mockTokenStore is a test double implementing auth.TokenStore.
type mockTokenStore struct {
	getByHash      func(ctx context.Context, hash string) (*auth.TokenRecord, error)
	updateLastUsed func(ctx context.Context, id string) error
}

func (m *mockTokenStore) Create(ctx context.Context, userID, name, tokenHash string, expiresAt *time.Time) (*auth.TokenRecord, error) {
	return nil, nil
}

func (m *mockTokenStore) GetByHash(ctx context.Context, hash string) (*auth.TokenRecord, error) {
	return m.getByHash(ctx, hash)
}

func (m *mockTokenStore) ListByUser(ctx context.Context, userID string) ([]*auth.TokenRecord, error) {
	return nil, nil
}

func (m *mockTokenStore) Revoke(ctx context.Context, id, userID string) error {
	return nil
}

func (m *mockTokenStore) UpdateLastUsed(ctx context.Context, id string) error {
	if m.updateLastUsed != nil {
		return m.updateLastUsed(ctx, id)
	}
	return nil
}

// okHandler is a simple handler that returns 200.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestBearerTokenMiddleware_ValidToken(t *testing.T) {
	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	testUser := &store.User{
		ID:    "user-1",
		Email: "test@example.com",
		Role:  "user",
	}

	ts := &mockTokenStore{
		getByHash: func(ctx context.Context, h string) (*auth.TokenRecord, error) {
			if h == hash {
				return &auth.TokenRecord{
					ID:        "token-1",
					UserID:    "user-1",
					TokenHash: hash,
				}, nil
			}
			return nil, store.ErrNotFound
		},
	}

	testDB := setupTestDBWithUser(t, testUser)
	us := store.NewUserStore(testDB)

	mw := auth.NewBearerTokenMiddleware(ts, us)
	handler := mw.Authenticate(okHandler())

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBearerTokenMiddleware_MissingHeader(t *testing.T) {
	ts := &mockTokenStore{
		getByHash: func(ctx context.Context, h string) (*auth.TokenRecord, error) {
			return nil, store.ErrNotFound
		},
	}
	us := store.NewUserStore(nil) // won't be called

	mw := auth.NewBearerTokenMiddleware(ts, us)
	handler := mw.Authenticate(okHandler())

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenMiddleware_InvalidToken(t *testing.T) {
	ts := &mockTokenStore{
		getByHash: func(ctx context.Context, h string) (*auth.TokenRecord, error) {
			return nil, store.ErrNotFound
		},
	}
	us := store.NewUserStore(nil)

	mw := auth.NewBearerTokenMiddleware(ts, us)
	handler := mw.Authenticate(okHandler())

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenMiddleware_RevokedToken(t *testing.T) {
	plaintext, hash, _ := auth.GenerateToken()
	now := time.Now()

	ts := &mockTokenStore{
		getByHash: func(ctx context.Context, h string) (*auth.TokenRecord, error) {
			if h == hash {
				return &auth.TokenRecord{
					ID:        "token-1",
					UserID:    "user-1",
					TokenHash: hash,
					RevokedAt: sql.NullTime{Time: now, Valid: true},
				}, nil
			}
			return nil, store.ErrNotFound
		},
	}
	us := store.NewUserStore(nil)

	mw := auth.NewBearerTokenMiddleware(ts, us)
	handler := mw.Authenticate(okHandler())

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenMiddleware_ExpiredToken(t *testing.T) {
	plaintext, hash, _ := auth.GenerateToken()
	expired := time.Now().Add(-1 * time.Hour)

	testUser := &store.User{
		ID:    "user-1",
		Email: "test@example.com",
		Role:  "user",
	}

	ts := &mockTokenStore{
		getByHash: func(ctx context.Context, h string) (*auth.TokenRecord, error) {
			if h == hash {
				return &auth.TokenRecord{
					ID:        "token-1",
					UserID:    testUser.ID,
					TokenHash: hash,
					ExpiresAt: sql.NullTime{Time: expired, Valid: true},
				}, nil
			}
			return nil, store.ErrNotFound
		},
	}
	us := store.NewUserStore(nil)

	mw := auth.NewBearerTokenMiddleware(ts, us)
	handler := mw.Authenticate(okHandler())

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenMiddleware_EmptyBearerValue(t *testing.T) {
	ts := &mockTokenStore{
		getByHash: func(ctx context.Context, h string) (*auth.TokenRecord, error) {
			return nil, store.ErrNotFound
		},
	}
	us := store.NewUserStore(nil)

	mw := auth.NewBearerTokenMiddleware(ts, us)
	handler := mw.Authenticate(okHandler())

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// setupTestDBWithUser creates an in-memory SQLite DB with migrations and seeds a user.
func setupTestDBWithUser(t *testing.T, user *store.User) *sqlx.DB {
	t.Helper()
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO users (id, provider, subject, email, display_name, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, "test", "sub-"+user.ID, user.Email, user.DisplayName, user.Role, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return db
}
