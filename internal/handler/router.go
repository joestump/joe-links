// Governing: SPEC-0001 REQ "Go HTTP Server", "Role-Based Access Control", "Short Link Resolution", ADR-0001, ADR-0003
// Governing: SPEC-0003 REQ "HTMX Theme Endpoint", ADR-0006
// Governing: SPEC-0004 REQ "Route Registration and Priority", "Shared Base Layout"
// Governing: SPEC-0005 REQ "API Router Mounting", ADR-0008
// Governing: SPEC-0007 REQ "Swagger UI Endpoint", ADR-0010
package handler

import (
	"io/fs"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joestump/joe-links/internal/api"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/web"
	_ "github.com/joestump/joe-links/docs/swagger"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Deps holds all dependencies required to build the HTTP router.
type Deps struct {
	SessionManager *scs.SessionManager
	AuthHandlers   *auth.Handlers
	AuthMiddleware *auth.Middleware
	LinkStore      *store.LinkStore
	OwnershipStore *store.OwnershipStore
	TagStore       *store.TagStore
	UserStore      *store.UserStore
	TokenStore     auth.TokenStore
	KeywordStore   *store.KeywordStore
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
	dashboard := NewDashboardHandler(deps.LinkStore, deps.TagStore)
	links := NewLinksHandler(deps.LinkStore, deps.OwnershipStore, deps.UserStore)
	tags := NewTagsHandler(deps.TagStore, deps.LinkStore)
	tokensWeb := NewTokensHandler(deps.TokenStore)

	r.Group(func(r chi.Router) {
		r.Use(deps.AuthMiddleware.RequireAuth)

		r.Get("/dashboard", dashboard.Show)

		// NOTE: validate-slug MUST be before /{id} to avoid chi treating "validate-slug" as an id
		r.Get("/dashboard/links/validate-slug", links.ValidateSlug)
		r.Get("/dashboard/links/new", links.New)
		r.Post("/dashboard/links", links.Create)
		r.Get("/dashboard/links/{id}", links.Detail)
		r.Get("/dashboard/links/{id}/edit", links.Edit)
		// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
		r.Get("/dashboard/links/{id}/confirm-delete", links.ConfirmDelete)
		r.Put("/dashboard/links/{id}", links.Update)
		r.Delete("/dashboard/links/{id}", links.Delete)
		r.Post("/dashboard/links/{id}/owners", links.AddOwner)
		r.Delete("/dashboard/links/{id}/owners/{uid}", links.RemoveOwner)

		r.Get("/dashboard/tags", tags.Index)
		r.Get("/dashboard/tags/suggest", tags.Suggest)
		r.Get("/dashboard/tags/{slug}", tags.Detail)

		// Governing: SPEC-0006 REQ "Token Management Web UI"
		r.Get("/dashboard/settings/tokens", tokensWeb.Index)
		r.Post("/dashboard/settings/tokens", tokensWeb.Create)
		// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
		r.Get("/dashboard/settings/tokens/{id}/confirm-revoke", tokensWeb.ConfirmRevoke)
		r.Delete("/dashboard/settings/tokens/{id}", tokensWeb.Revoke)
	})

	// Admin routes (require admin role)
	// Governing: SPEC-0004 REQ "Route Registration and Priority" — admin group with RequireAdmin
	admin := NewAdminHandler(deps.LinkStore, deps.UserStore, deps.KeywordStore)
	keywordsHandler := NewKeywordsHandler(deps.KeywordStore)
	r.Group(func(r chi.Router) {
		r.Use(deps.AuthMiddleware.RequireAuth)
		r.Use(deps.AuthMiddleware.RequireRole("admin"))
		r.Get("/admin", admin.Dashboard)
		r.Get("/admin/users", admin.Users)
		// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
		r.Get("/admin/users/{id}/confirm-delete", admin.ConfirmDeleteUser)
		r.Put("/admin/users/{id}/role", admin.UpdateRole)
		// Governing: SPEC-0011 REQ "Admin Links Screen", "Admin Inline Link Editing", "Admin Link Deletion"
		r.Get("/admin/links", admin.Links)
		r.Get("/admin/links/{id}/edit", admin.EditLinkRow)
		r.Get("/admin/links/{id}/row", admin.LinkRow)
		r.Put("/admin/links/{id}", admin.UpdateLink)
		r.Get("/admin/links/{id}/confirm-delete", admin.ConfirmDeleteLink)
		r.Delete("/admin/links/{id}", admin.DeleteLink)

		// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
		r.Get("/admin/keywords", keywordsHandler.Index)
		r.Post("/admin/keywords", keywordsHandler.Create)
		// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
		r.Get("/admin/keywords/{id}/confirm-delete", keywordsHandler.ConfirmDelete)
		r.Delete("/admin/keywords/{id}", keywordsHandler.Delete)
	})

	// Swagger UI — no auth required; MUST be before slug catch-all.
	// Governing: SPEC-0007 REQ "Swagger UI Endpoint", REQ "Swagger UI Authorization"
	r.Get("/api/docs/*", httpSwagger.WrapHandler)

	// API sub-router at /api/v1 — must be before slug catch-all.
	// Governing: SPEC-0005 REQ "API Router Mounting"
	tokenStore := deps.TokenStore
	bearerMiddleware := auth.NewBearerTokenMiddleware(tokenStore, deps.UserStore)
	apiRouter := api.NewAPIRouter(api.Deps{
		BearerMiddleware: bearerMiddleware,
		TokenStore:       tokenStore,
		LinkStore:        deps.LinkStore,
		OwnershipStore:   deps.OwnershipStore,
		TagStore:         deps.TagStore,
		UserStore:        deps.UserStore,
		KeywordStore:     deps.KeywordStore,
	})
	r.Mount("/api/v1", apiRouter)

	// Slug resolver -- catch-all, must be last.
	// Resolver does not require auth (links are publicly accessible).
	// Uses OptionalUser so the 404 page can offer "Create this link" when logged in.
	// Governing: SPEC-0004 REQ "Route Registration and Priority" — catch-all AFTER named routes
	// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", ADR-0013 — wildcard for multi-segment paths
	resolver := NewResolveHandler(deps.LinkStore, deps.KeywordStore)
	r.With(deps.AuthMiddleware.OptionalUser).Get("/{slug}*", resolver.Resolve)

	return r
}
