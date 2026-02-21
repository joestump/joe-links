// Governing: SPEC-0001 REQ "Short Link Management", ADR-0001
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// LinkForm holds form input values for creating or editing a link.
type LinkForm struct {
	Slug        string
	URL         string
	Description string
}

// LinkFormPage is the template data for the new/edit link forms.
type LinkFormPage struct {
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
func (h *LinksHandler) New(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	form := LinkForm{Slug: r.URL.Query().Get("slug")}
	render(w, "new.html", LinkFormPage{User: user, Form: form})
}

// Create processes the create-link form submission.
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
		render(w, "new.html", LinkFormPage{User: user, Form: form, Error: err.Error()})
		return
	}

	_, err := h.links.Create(r.Context(), form.Slug, form.URL, user.ID, "", form.Description)
	if err != nil {
		render(w, "new.html", LinkFormPage{User: user, Form: form, Error: "That slug is already taken. Choose a different one."})
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Edit renders the edit-link form.
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

	render(w, "edit.html", LinkFormPage{User: user, Link: link})
}

// Update processes the edit-link form submission.
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

	form := LinkForm{
		Slug:        r.FormValue("slug"),
		URL:         r.FormValue("url"),
		Description: r.FormValue("description"),
	}

	if err := store.ValidateSlugFormat(form.Slug); err != nil {
		render(w, "edit.html", LinkFormPage{User: user, Link: link, Form: form, Error: err.Error()})
		return
	}

	_, err = h.links.Update(r.Context(), id, form.Slug, form.URL, "", form.Description)
	if err != nil {
		render(w, "edit.html", LinkFormPage{User: user, Link: link, Form: form, Error: "That slug is already taken."})
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Delete removes a link. Returns 200 with empty body for HTMX row removal.
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
