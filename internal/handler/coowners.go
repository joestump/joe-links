// Governing: SPEC-0004 REQ "Co-Owner Management", ADR-0007
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// AddOwner handles POST /dashboard/links/{id}/owners.
func (h *LinksHandler) AddOwner(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// RemoveOwner handles DELETE /dashboard/links/{id}/owners/{uid}.
func (h *LinksHandler) RemoveOwner(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = chi.URLParam(r, "uid")
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Detail handles GET /dashboard/links/{id}.
func (h *LinksHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Link: link}
	if isHTMX(r) {
		renderFragment(w, "content", data)
		return
	}
	render(w, "links/detail.html", data)
}

// ValidateSlug handles GET /dashboard/links/validate-slug?slug=...
func (h *LinksHandler) ValidateSlug(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	w.Header().Set("Content-Type", "text/html")
	if err := store.ValidateSlugFormat(slug); err != nil {
		w.Write([]byte(`<span class="text-error text-xs">` + err.Error() + `</span>`))
		return
	}
	if _, err := h.links.GetBySlug(r.Context(), slug); err == nil {
		w.Write([]byte(`<span class="text-error text-xs">Slug already taken</span>`))
		return
	}
	w.Write([]byte(`<span class="text-success text-xs">Available!</span>`))
}
