// Governing: SPEC-0001 REQ "Short Link Resolution", ADR-0001
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// ResolveHandler handles short link slug resolution and redirection.
type ResolveHandler struct {
	links *store.LinkStore
}

// NewResolveHandler creates a new ResolveHandler.
func NewResolveHandler(ls *store.LinkStore) *ResolveHandler {
	return &ResolveHandler{links: ls}
}

type notFoundPage struct {
	User  *store.User
	Slug  string
	Flash *Flash
}

// Resolve looks up a slug and redirects to the target URL, or renders a 404 page.
func (h *ResolveHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	link, err := h.links.GetBySlug(r.Context(), slug)
	if err != nil {
		user := auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusNotFound)
		render(w, "404.html", notFoundPage{User: user, Slug: slug})
		return
	}
	http.Redirect(w, r, link.URL, http.StatusFound)
}
