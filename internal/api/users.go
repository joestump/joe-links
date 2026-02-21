// Governing: SPEC-0005 REQ "User Profile"
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
)

// usersAPIHandler provides REST handlers for user endpoints.
// Governing: SPEC-0005 REQ "User Profile"
type usersAPIHandler struct{}

// registerUserRoutes registers user routes on r.
// Governing: SPEC-0005 REQ "User Profile"
func registerUserRoutes(r chi.Router) {
	h := &usersAPIHandler{}
	r.Get("/users/me", h.Me)
}

// Me returns the authenticated caller's profile.
// GET /api/v1/users/me
// Governing: SPEC-0005 REQ "User Profile" — returns id, email, display_name, role, created_at.
func (h *usersAPIHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, &UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
	})
}
