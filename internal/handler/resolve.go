// Governing: SPEC-0001 REQ "Short Link Resolution", REQ "HTMX Hypermedia Interactions", ADR-0001
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
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *ResolveHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	link, err := h.links.GetBySlug(r.Context(), slug)
	if err != nil {
		user := auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusNotFound)
		data := notFoundPage{User: user, Slug: slug}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "404.html", data)
		return
	}
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", link.URL)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, link.URL, http.StatusFound)
}
