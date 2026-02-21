// Governing: SPEC-0001 REQ "Short Link Management", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
package handler

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

var slugRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

// Governing: SPEC-0001 REQ "Short Link Resolution" — reserved prefixes MUST NOT be valid slugs.
var reservedPrefixes = []string{"auth", "static", "dashboard", "admin"}

// isReservedSlug returns true if the slug matches or starts with a reserved prefix.
func isReservedSlug(slug string) bool {
	for _, prefix := range reservedPrefixes {
		if slug == prefix || strings.HasPrefix(slug, prefix+"-") {
			return true
		}
	}
	return false
}

// LinkForm holds form input values for creating or editing a link.
type LinkForm struct {
	Slug        string
	URL         string
	Description string
}

// LinkFormPage is the template data for the new/edit link forms.
type LinkFormPage struct {
	BasePage
	User  *store.User
	Link  *store.Link
	Form  LinkForm
	Error string
	Flash *Flash
}

// LinksHandler provides HTTP handlers for link CRUD operations.
type LinksHandler struct {
	links *store.LinkStore
	owns  *store.OwnershipStore
}

// NewLinksHandler creates a new LinksHandler.
func NewLinksHandler(ls *store.LinkStore, os *store.OwnershipStore) *LinksHandler {
	return &LinksHandler{links: ls, owns: os}
}

// New renders the create-link form.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *LinksHandler) New(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	form := LinkForm{Slug: r.URL.Query().Get("slug")}
	data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Form: form}
	if isHTMX(r) {
		renderFragment(w, "content", data)
		return
	}
	render(w, "new.html", data)
}

// Create processes the create-link form submission.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *LinksHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form := LinkForm{
		Slug:        r.FormValue("slug"),
		URL:         r.FormValue("url"),
		Description: r.FormValue("description"),
	}

	if err := store.ValidateSlugFormat(form.Slug); err != nil {
		data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Form: form, Error: err.Error()}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "new.html", data)
		return
	}
	if isReservedSlug(form.Slug) {
		render(w, "new.html", LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Form: form, Error: "That slug uses a reserved prefix (auth, static, dashboard, admin)."})
		return
	}

	_, err := h.links.Create(r.Context(), form.Slug, form.URL, user.ID, "", form.Description)
	if err != nil {
		data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Form: form, Error: "That slug is already taken. Choose a different one."}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "new.html", data)
		return
	}

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Edit renders the edit-link form.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *LinksHandler) Edit(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Governing: SPEC-0002 REQ "Authorization Based on Ownership"
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
	render(w, "edit.html", data)
}

// Update processes the edit-link form submission.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *LinksHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Governing: SPEC-0002 REQ "Authorization Based on Ownership"
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Governing: SPEC-0001 REQ "Short Link Management" — slug is immutable after creation.
	form := LinkForm{
		URL:         r.FormValue("url"),
		Description: r.FormValue("description"),
	}

	// Governing: SPEC-0001 REQ "Short Link Management" — slug is immutable, not updated here.
	_, err = h.links.Update(r.Context(), id, form.URL, "", form.Description)
	if err != nil {
		data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Link: link, Form: form, Error: "Update failed."}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "edit.html", data)
		return
	}

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Delete removes a link. Returns 200 with empty body for HTMX row removal.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *LinksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Governing: SPEC-0002 REQ "Authorization Based on Ownership"
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.links.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
