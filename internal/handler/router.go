// Governing: SPEC-0001 REQ "Go HTTP Server", ADR-0001
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
}

// NewRouter assembles the full chi router with all middleware and routes.
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

	// Authenticated routes
	dashboard := NewDashboardHandler(deps.LinkStore)
	links := NewLinksHandler(deps.LinkStore)

	r.Group(func(r chi.Router) {
		r.Use(deps.AuthMiddleware.RequireAuth)

		r.Get("/dashboard", dashboard.Show)
		r.Get("/links/new", links.New)
		r.Post("/links", links.Create)
		r.Get("/links/{id}/edit", links.Edit)
		r.Put("/links/{id}", links.Update)
		r.Delete("/links/{id}", links.Delete)
	})

	// Slug resolver -- catch-all, must be last.
	// Resolver does not require auth (links are publicly accessible).
	resolver := NewResolveHandler(deps.LinkStore)
	r.Get("/{slug}", resolver.Resolve)

	// Root redirect
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	return r
}
