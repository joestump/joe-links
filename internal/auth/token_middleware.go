// Governing: SPEC-0006 REQ "Bearer Token Middleware", REQ "No Web UI Session on API Routes", ADR-0009
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/joestump/joe-links/internal/store"
)

// BearerTokenMiddleware authenticates API requests via Bearer token.
// It explicitly rejects session cookies — only API tokens are accepted.
// Governing: SPEC-0006 REQ "No Web UI Session on API Routes"
type BearerTokenMiddleware struct {
	tokens TokenStore
	users  *store.UserStore
}

// NewBearerTokenMiddleware creates a new BearerTokenMiddleware.
func NewBearerTokenMiddleware(ts TokenStore, us *store.UserStore) *BearerTokenMiddleware {
	return &BearerTokenMiddleware{tokens: ts, users: us}
}

// Authenticate is an http.Handler middleware that extracts and validates a Bearer token.
// WHEN valid: injects the token owner's *store.User into context and fires an async last_used_at update.
// WHEN invalid/missing/expired/revoked: returns 401 with {"error": "unauthorized"}.
// Governing: SPEC-0006 REQ "Bearer Token Middleware"
func (m *BearerTokenMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Bearer token from Authorization header.
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w)
			return
		}
		plaintext := strings.TrimPrefix(authHeader, "Bearer ")
		if plaintext == "" {
			writeUnauthorized(w)
			return
		}

		// Hash the plaintext token and look it up.
		hash := HashToken(plaintext)
		rec, err := m.tokens.GetByHash(r.Context(), hash)
		if err != nil {
			writeUnauthorized(w)
			return
		}

		// Reject revoked tokens.
		// Governing: SPEC-0006 REQ "Bearer Token Middleware" — revoked_at IS NULL
		if rec.RevokedAt.Valid {
			writeUnauthorized(w)
			return
		}

		// Reject expired tokens.
		// Governing: SPEC-0006 REQ "Bearer Token Middleware" — expires_at IS NULL OR expires_at > NOW()
		if rec.ExpiresAt.Valid && rec.ExpiresAt.Time.Before(time.Now()) {
			writeUnauthorized(w)
			return
		}

		// Load the user who owns the token.
		user, err := m.users.GetByID(r.Context(), rec.UserID)
		if err != nil {
			writeUnauthorized(w)
			return
		}

		// Update last_used_at asynchronously to avoid write overhead on every read.
		// Governing: ADR-0009 (async last_used_at)
		go m.tokens.UpdateLastUsed(context.Background(), rec.ID)

		// Inject user into context using the same key as session-based auth.
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeUnauthorized writes a 401 JSON response with {"error": "unauthorized"}.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
