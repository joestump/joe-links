// Governing: SPEC-0005 REQ "API Router Mounting", ADR-0008
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// Deps holds all dependencies required to build the API router.
type Deps struct {
	BearerAuth     *auth.BearerTokenMiddleware
	LinkStore      *store.LinkStore
	OwnershipStore *store.OwnershipStore
	TagStore       *store.TagStore
	UserStore      *store.UserStore
	TokenStore     auth.TokenStore
}

// NewAPIRouter creates a chi sub-router for /api/v1.
// All routes require Bearer token authentication and return application/json.
// Governing: SPEC-0005 REQ "API Router Mounting", ADR-0008
func NewAPIRouter(deps Deps) chi.Router {
	r := chi.NewRouter()

	// All API responses are JSON.
	// Governing: SPEC-0005 REQ "API Router Mounting" — all routes MUST return Content-Type: application/json
	r.Use(jsonContentType)

	// Bearer token authentication on all API routes.
	// Governing: SPEC-0005 REQ "API Router Mounting" — BearerTokenMiddleware MUST be applied
	r.Use(deps.BearerAuth.Authenticate)

	// Placeholder: link routes will be added by Issue #46
	// Placeholder: token routes will be added by Issue #44
	// Placeholder: tag and user profile routes will be added by Issue #47
	// Placeholder: admin routes will be added by Issue #48

	return r
}

// jsonContentType is a middleware that sets Content-Type: application/json on all responses.
// Governing: SPEC-0005 REQ "API Router Mounting"
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
