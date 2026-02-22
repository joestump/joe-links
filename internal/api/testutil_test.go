package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/joestump/joe-links/internal/api"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

// testEnv holds all stores and helpers needed for API integration tests.
type testEnv struct {
	Router         http.Handler
	LinkStore      *store.LinkStore
	TagStore       *store.TagStore
	OwnershipStore *store.OwnershipStore
	UserStore      *store.UserStore
	TokenStore     *auth.SQLTokenStore
}

// newTestEnv creates an in-memory SQLite test database, runs migrations,
// and wires up the full API router with real stores.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := testutil.NewTestDB(t)

	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	ts := auth.NewSQLTokenStore(db)

	bearerMW := auth.NewBearerTokenMiddleware(ts, us)

	deps := api.Deps{
		BearerMiddleware: bearerMW,
		TokenStore:       ts,
		LinkStore:        ls,
		OwnershipStore:   owns,
		TagStore:         tags,
		UserStore:        us,
	}

	router := api.NewAPIRouter(deps)
	return &testEnv{
		Router:         router,
		LinkStore:      ls,
		TagStore:       tags,
		OwnershipStore: owns,
		UserStore:      us,
		TokenStore:     ts,
	}
}

// seedUser creates a user and returns the user record.
func seedUser(t *testing.T, env *testEnv, email, role string) *store.User {
	t.Helper()
	ctx := context.Background()
	u, err := env.UserStore.Upsert(ctx, "test", "sub-"+email, email, "Test User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if role != "user" {
		u, err = env.UserStore.UpdateRole(ctx, u.ID, role)
		if err != nil {
			t.Fatalf("update role: %v", err)
		}
	}
	return u
}

// seedToken creates a real API token for a user and returns the plaintext Bearer value.
func seedToken(t *testing.T, env *testEnv, userID string) string {
	t.Helper()
	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	_, err = env.TokenStore.Create(context.Background(), userID, "test-token", hash, nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return plaintext
}


// authRequest adds a Bearer token to the request.
func authRequest(r *http.Request, token string) *http.Request {
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}
