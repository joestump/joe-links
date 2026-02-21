// Governing: SPEC-0005 REQ "API Router Mounting", ADR-0008
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// Deps holds dependencies for the API router.
type Deps struct {
	BearerMiddleware *auth.BearerTokenMiddleware
	TokenStore       auth.TokenStore
	LinkStore        *store.LinkStore
	OwnershipStore   *store.OwnershipStore
	TagStore         *store.TagStore
	UserStore        *store.UserStore
}

// NewAPIRouter creates and returns a chi router for /api/v1.
// The caller mounts it at /api/v1 in the main router.
// Governing: SPEC-0005 REQ "API Router Mounting", ADR-0008
func NewAPIRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	// Enforce JSON content type on all API responses.
	// Governing: SPEC-0005 REQ "API Router Mounting"
	r.Use(jsonContentType)

	// Bearer token authentication — required on all /api/v1/* routes.
	// Governing: SPEC-0006 REQ "No Web UI Session on API Routes"
	r.Use(deps.BearerMiddleware.Authenticate)

	// Token management routes.
	// Governing: SPEC-0006 REQ "Token Management API"
	registerTokenRoutes(r, deps.TokenStore)

	// Tag routes.
	// Governing: SPEC-0005 REQ "Tags"
	registerTagRoutes(r, deps.TagStore, deps.LinkStore)

	// User profile routes.
	// Governing: SPEC-0005 REQ "User Profile"
	registerUserRoutes(r)

	// Link and co-owner management routes.
	// Governing: SPEC-0005 REQ "Links Collection", REQ "Link Resource", REQ "Co-Owner Management"
	registerLinkRoutes(r, deps.LinkStore, deps.OwnershipStore, deps.UserStore)

	// Admin-only routes behind role-check middleware group.
	// Governing: SPEC-0005 REQ "Admin Endpoints", ADR-0008
	registerAdminRoutes(r, deps.UserStore, deps.LinkStore, deps.OwnershipStore)

	return r
}

// jsonContentType middleware sets Content-Type: application/json on all responses.
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
