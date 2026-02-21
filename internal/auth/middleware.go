// Governing: SPEC-0001 REQ "Role-Based Access Control", ADR-0003
package auth

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/joestump/joe-links/internal/store"
)

type contextKey string

const UserContextKey contextKey = "user"

// Middleware provides HTTP middleware for authentication and authorization.
type Middleware struct {
	sessions *scs.SessionManager
	users    *store.UserStore
}

// NewMiddleware creates a new auth Middleware.
func NewMiddleware(sm *scs.SessionManager, us *store.UserStore) *Middleware {
	return &Middleware{sessions: sm, users: us}
}

// RequireAuth redirects to /auth/login if no valid session exists.
// On success, sets the *store.User on the request context.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := m.sessions.GetString(r.Context(), SessionUserIDKey)
		if userID == "" {
			http.Redirect(w, r, "/auth/login?redirect="+r.URL.RequestURI(), http.StatusFound)
			return
		}

		user, err := m.users.GetByID(r.Context(), userID)
		if err != nil {
			// Session references a deleted user — destroy and redirect
			_ = m.sessions.Destroy(r.Context())
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns a middleware that requires the user to have the given role.
// Must be used after RequireAuth.
func (m *Middleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserContextKey).(*store.User)
			if !ok || user.Role != role {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserFromContext retrieves the authenticated user from the context.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(UserContextKey).(*store.User)
	return u
}
