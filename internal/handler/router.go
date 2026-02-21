// Governing: SPEC-0001 REQ "Go HTTP Server", "Role-Based Access Control", "Short Link Resolution", ADR-0001, ADR-0003
// Governing: SPEC-0003 REQ "HTMX Theme Endpoint", ADR-0006
// Governing: SPEC-0004 REQ "Route Registration and Priority", "Shared Base Layout"
package handler

import (
	"io/fs"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/web"
)

// Deps holds all dependencies required to build the HTTP router.
type Deps struct {
	SessionManager *scs.SessionManager
	AuthHandlers   *auth.Handlers
	AuthMiddleware *auth.Middleware
	LinkStore      *store.LinkStore
	OwnershipStore *store.OwnershipStore
	TagStore       *store.TagStore
}

// NewRouter assembles the full chi router with all middleware and routes.
// Governing: SPEC-0004 REQ "Route Registration and Priority" — named routes registered
// before catch-all slug resolver; reserved prefixes take precedence.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(deps.SessionManager.LoadAndSave)

	// Static assets (embedded). Use fs.Sub so the file server sees
	// css/app.css and js/htmx.min.js directly, not static/css/... paths.
	staticSub, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		panic("failed to sub static FS: " + err.Error())
	}
	r.Handle("/static/*", http.StripPrefix("/static", http.FileServerFS(staticSub)))

	// Auth routes (no auth required)
	r.Get("/auth/login", deps.AuthHandlers.Login)
	r.Get("/auth/callback", deps.AuthHandlers.Callback)
	r.Post("/auth/logout", deps.AuthHandlers.Logout)

	// Theme toggle — no auth required, must precede auth group.
	// Governing: SPEC-0003 REQ "HTMX Theme Endpoint"
	themeHandler := NewThemeHandler()
	r.Post("/dashboard/theme", themeHandler.Toggle)

	// Landing page (unauthenticated; redirects authenticated to /dashboard)
	// Uses OptionalUser so we can detect logged-in users without requiring auth.
	// Governing: SPEC-0004 REQ "Landing Page"
	landing := NewLandingHandler()
	r.With(deps.AuthMiddleware.OptionalUser).Get("/", landing.Index)

	// Authenticated routes
	// Governing: SPEC-0004 REQ "Route Registration and Priority" — dashboard, link, and tag routes
	dashboard := NewDashboardHandler(deps.LinkStore)
	links := NewLinksHandler(deps.LinkStore, deps.OwnershipStore)
	tags := NewTagsHandler(deps.TagStore)

	r.Group(func(r chi.Router) {
		r.Use(deps.AuthMiddleware.RequireAuth)

		r.Get("/dashboard", dashboard.Show)

		// NOTE: validate-slug MUST be before /{id} to avoid chi treating "validate-slug" as an id
		r.Get("/dashboard/links/validate-slug", links.ValidateSlug)
		r.Get("/dashboard/links/new", links.New)
		r.Post("/dashboard/links", links.Create)
		r.Get("/dashboard/links/{id}", links.Detail)
		r.Get("/dashboard/links/{id}/edit", links.Edit)
		r.Put("/dashboard/links/{id}", links.Update)
		r.Delete("/dashboard/links/{id}", links.Delete)
		r.Post("/dashboard/links/{id}/owners", links.AddOwner)
		r.Delete("/dashboard/links/{id}/owners/{uid}", links.RemoveOwner)

		r.Get("/dashboard/tags", tags.Index)
		r.Get("/dashboard/tags/suggest", tags.Suggest)
		r.Get("/dashboard/tags/{slug}", tags.Detail)
	})

	// Admin routes (require admin role)
	// Governing: SPEC-0004 REQ "Route Registration and Priority" — admin group with RequireAdmin
	admin := NewAdminHandler()
	r.Group(func(r chi.Router) {
		r.Use(deps.AuthMiddleware.RequireAuth)
		r.Use(deps.AuthMiddleware.RequireRole("admin"))
		r.Get("/admin", admin.Dashboard)
		r.Get("/admin/users", admin.Users)
		r.Put("/admin/users/{id}/role", admin.UpdateRole)
		r.Get("/admin/links", admin.Links)
	})

	// Slug resolver -- catch-all, must be last.
	// Resolver does not require auth (links are publicly accessible).
	// Uses OptionalUser so the 404 page can offer "Create this link" when logged in.
	// Governing: SPEC-0004 REQ "Route Registration and Priority" — catch-all AFTER named routes
	resolver := NewResolveHandler(deps.LinkStore)
	r.With(deps.AuthMiddleware.OptionalUser).Get("/{slug}", resolver.Resolve)

	return r
}
