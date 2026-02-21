// Governing: SPEC-0001 REQ "Short Link Management", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
// Governing: SPEC-0004 REQ "New Link Form", "Edit Link Form", "Delete Link"
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
	Title       string
	Description string
	Tags        string // comma-separated tag names
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

// LinkDetailPage is the template data for the link detail view.
// Governing: SPEC-0004 REQ "Link Detail View"
type LinkDetailPage struct {
	BasePage
	User   *store.User
	Link   *store.Link
	Tags   []*store.Tag
	Owners []*store.OwnerInfo
}

// LinksHandler provides HTTP handlers for link CRUD operations.
type LinksHandler struct {
	links *store.LinkStore
	owns  *store.OwnershipStore
	users *store.UserStore
}

// NewLinksHandler creates a new LinksHandler.
func NewLinksHandler(ls *store.LinkStore, os *store.OwnershipStore, us *store.UserStore) *LinksHandler {
	return &LinksHandler{links: ls, owns: os, users: us}
}

// New renders the create-link form.
// Governing: SPEC-0004 REQ "New Link Form"
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
// Governing: SPEC-0004 REQ "New Link Form"
func (h *LinksHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form := LinkForm{
		Slug:        r.FormValue("slug"),
		URL:         r.FormValue("url"),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Tags:        r.FormValue("tags"),
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
		data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Form: form, Error: "That slug uses a reserved prefix (auth, static, dashboard, admin)."}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "new.html", data)
		return
	}

	link, err := h.links.Create(r.Context(), form.Slug, form.URL, user.ID, form.Title, form.Description)
	if err != nil {
		data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Form: form, Error: "That slug is already taken. Choose a different one."}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "new.html", data)
		return
	}

	// Set tags if provided
	if form.Tags != "" {
		tagNames := parseTagNames(form.Tags)
		if len(tagNames) > 0 {
			_ = h.links.SetTags(r.Context(), link.ID, tagNames)
		}
	}

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Edit renders the edit-link form.
// Governing: SPEC-0004 REQ "Edit Link Form"
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

	// Load current tags for pre-fill
	tags, _ := h.links.ListTags(r.Context(), link.ID)
	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Name
	}

	form := LinkForm{
		URL:         link.URL,
		Title:       link.Title,
		Description: link.Description,
		Tags:        strings.Join(tagNames, ", "),
	}

	data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Link: link, Form: form}
	if isHTMX(r) {
		renderFragment(w, "content", data)
		return
	}
	render(w, "edit.html", data)
}

// Update processes the edit-link form submission.
// Governing: SPEC-0004 REQ "Edit Link Form"
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
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Tags:        r.FormValue("tags"),
	}

	_, err = h.links.Update(r.Context(), id, form.URL, form.Title, form.Description)
	if err != nil {
		data := LinkFormPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Link: link, Form: form, Error: "Update failed."}
		if isHTMX(r) {
			renderFragment(w, "content", data)
			return
		}
		render(w, "edit.html", data)
		return
	}

	// Update tags
	tagNames := parseTagNames(form.Tags)
	_ = h.links.SetTags(r.Context(), id, tagNames)

	redirect := "/dashboard/links/" + id
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", redirect)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// Delete removes a link. Returns 200 with empty body for HTMX row removal.
// Governing: SPEC-0004 REQ "Delete Link"
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

	// Governing: SPEC-0004 REQ "Delete Link" — OOB toast on success
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<div id="toast-area" hx-swap-oob="innerHTML:#toast-area"><div class="alert alert-success"><span>Link deleted.</span></div></div>`))
}

// parseTagNames splits a comma-separated string into trimmed, non-empty tag names.
func parseTagNames(s string) []string {
	var names []string
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
