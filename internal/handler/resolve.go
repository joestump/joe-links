// Governing: SPEC-0001 REQ "Short Link Resolution", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
// Governing: SPEC-0004 REQ "Slug Resolver and 404 Page"
package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// ResolveHandler handles short link slug resolution and redirection.
type ResolveHandler struct {
	links    *store.LinkStore
	keywords *store.KeywordStore
}

// NewResolveHandler creates a new ResolveHandler.
func NewResolveHandler(ls *store.LinkStore, ks *store.KeywordStore) *ResolveHandler {
	return &ResolveHandler{links: ls, keywords: ks}
}

type notFoundPage struct {
	BasePage
	User  *store.User
	Slug  string
	Flash *Flash
}

// Resolve looks up a slug and redirects to the target URL, or renders a 404 page.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *ResolveHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// Governing: ADR-0011 — check if request host is a registered keyword.
	host := strings.SplitN(r.Host, ":", 2)[0]
	kw, kwErr := h.keywords.GetByKeyword(r.Context(), host)
	if kwErr == nil {
		// Substitute {slug} in the URL template and redirect.
		target := strings.ReplaceAll(kw.URLTemplate, "{slug}", slug)
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	// kwErr == store.ErrNotFound → fall through to normal slug resolution

	link, err := h.links.GetBySlug(r.Context(), slug)
	if err != nil {
		user := auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusNotFound)
		data := notFoundPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Slug: slug}
		if isHTMX(r) {
			renderPageFragment(w, "404.html", "content", data)
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
