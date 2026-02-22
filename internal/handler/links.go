// Governing: SPEC-0001 REQ "Short Link Management", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
// Governing: SPEC-0004 REQ "New Link Form", "Edit Link Form", "Delete Link"
// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
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
// Governing: SPEC-0012 REQ "User Profile Route Priority" — "u" reserved for user profile pages.
// Governing: SPEC-0012 REQ "Public Link Browser Route Priority" — "links" reserved for public link browser.
var reservedPrefixes = []string{"auth", "static", "dashboard", "admin", "u", "links"}

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
// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms"
type LinkForm struct {
	Slug        string
	URL         string
	Title       string
	Description string
	Tags        string // comma-separated tag names
	Visibility  string // public, private, or secure
}

// LinkFormPage is the template data for the new/edit link forms.
type LinkFormPage struct {
	BasePage
	User    *store.User
	Link    *store.Link
	Form    LinkForm
	Error   string
	Flash   *Flash
	Keyword string // first configured keyword (e.g. "go") for slug prefix display
}

// LinkDetailPage is the template data for the link detail view.
// Governing: SPEC-0004 REQ "Link Detail View"
// Governing: SPEC-0010 REQ "Share Management Panel on Link Detail"
type LinkDetailPage struct {
	BasePage
	User   *store.User
	Link   *store.Link
	Tags   []*store.Tag
	Owners []*store.OwnerInfo
	Shares []ShareUser
}

// ShareUser combines share record with user display info for templates.
// Governing: SPEC-0010 REQ "Share Management Panel on Link Detail"
type ShareUser struct {
	UserID      string
	DisplayName string
	Email       string
}

// ConfirmDeleteData holds template data for the delete confirmation modal.
// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
type ConfirmDeleteData struct {
	Name      string
	DeleteURL string
	Target    string
}

// LinksHandler provides HTTP handlers for link CRUD operations.
type LinksHandler struct {
	links    *store.LinkStore
	owns     *store.OwnershipStore
	users    *store.UserStore
	keywords *store.KeywordStore
}

// NewLinksHandler creates a new LinksHandler.
func NewLinksHandler(ls *store.LinkStore, os *store.OwnershipStore, us *store.UserStore, ks *store.KeywordStore) *LinksHandler {
	return &LinksHandler{links: ls, owns: os, users: us, keywords: ks}
}

// New renders the create-link form.
// Governing: SPEC-0004 REQ "New Link Form"
func (h *LinksHandler) New(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	form := LinkForm{Slug: r.URL.Query().Get("slug")}

	// Load first keyword for slug prefix display (e.g. "go" → "go/slug")
	keyword := ""
	if kws, _ := h.keywords.List(r.Context()); len(kws) > 0 {
		keyword = kws[0].Keyword
	}

	data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Form: form, Keyword: keyword}
	if isHTMX(r) {
		renderFragment(w, "new_link_modal", data)
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

	// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms"
	visibility := r.FormValue("visibility")
	if visibility == "" {
		visibility = "public"
	}

	form := LinkForm{
		Slug:        r.FormValue("slug"),
		URL:         r.FormValue("url"),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Tags:        r.FormValue("tags"),
		Visibility:  visibility,
	}

	// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms" — validate visibility value
	if err := store.ValidateVisibility(form.Visibility); err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Form: form, Error: err.Error()}
		if isHTMX(r) {
			renderFragment(w, "new_link_modal", data)
			return
		}
		render(w, "new.html", data)
		return
	}

	// Governing: SPEC-0013 REQ "Create/Edit Link Form as HTMX Modal" — validation errors re-render inside modal
	if err := store.ValidateSlugFormat(form.Slug); err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Form: form, Error: err.Error()}
		if isHTMX(r) {
			renderFragment(w, "new_link_modal", data)
			return
		}
		render(w, "new.html", data)
		return
	}
	if isReservedSlug(form.Slug) {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Form: form, Error: "That slug uses a reserved prefix (auth, static, dashboard, admin, links)."}
		if isHTMX(r) {
			renderFragment(w, "new_link_modal", data)
			return
		}
		render(w, "new.html", data)
		return
	}

	// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
	if err := store.ValidateURLVariables(form.URL); err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Form: form, Error: err.Error()}
		if isHTMX(r) {
			renderFragment(w, "new_link_modal", data)
			return
		}
		render(w, "new.html", data)
		return
	}

	link, err := h.links.Create(r.Context(), form.Slug, form.URL, user.ID, form.Title, form.Description, form.Visibility)
	if err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Form: form, Error: "That slug is already taken. Choose a different one."}
		if isHTMX(r) {
			renderFragment(w, "new_link_modal", data)
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

	// Governing: SPEC-0013 REQ "Create/Edit Link Form as HTMX Modal" — close modal + trigger list refresh
	if isHTMX(r) {
		w.Header().Set("HX-Trigger", "linkCreated")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Edit renders the edit-link form.
// Governing: SPEC-0004 REQ "Edit Link Form"
// Governing: SPEC-0013 REQ "Create/Edit Link Form as HTMX Modal"
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
		Visibility:  link.Visibility,
	}

	data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Link: link, Form: form}
	if isHTMX(r) {
		renderFragment(w, "edit_link_modal", data)
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
	// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms"
	visibility := r.FormValue("visibility")
	if visibility == "" {
		visibility = link.Visibility
	}

	form := LinkForm{
		URL:         r.FormValue("url"),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Tags:        r.FormValue("tags"),
		Visibility:  visibility,
	}

	// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms" — validate visibility value
	if err := store.ValidateVisibility(form.Visibility); err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Link: link, Form: form, Error: err.Error()}
		if isHTMX(r) {
			renderFragment(w, "edit_link_modal", data)
			return
		}
		render(w, "edit.html", data)
		return
	}

	// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
	if err := store.ValidateURLVariables(form.URL); err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Link: link, Form: form, Error: err.Error()}
		if isHTMX(r) {
			renderFragment(w, "edit_link_modal", data)
			return
		}
		render(w, "edit.html", data)
		return
	}

	_, err = h.links.Update(r.Context(), id, form.URL, form.Title, form.Description, form.Visibility)
	if err != nil {
		data := LinkFormPage{BasePage: newBasePage(r, user), User: user, Link: link, Form: form, Error: "Update failed."}
		// Governing: SPEC-0013 REQ "Create/Edit Link Form as HTMX Modal" — re-render inside modal on error
		if isHTMX(r) {
			renderFragment(w, "edit_link_modal", data)
			return
		}
		render(w, "edit.html", data)
		return
	}

	// Update tags
	tagNames := parseTagNames(form.Tags)
	_ = h.links.SetTags(r.Context(), id, tagNames)

	// Governing: SPEC-0013 REQ "Create/Edit Link Form as HTMX Modal" — close modal + trigger list refresh
	if isHTMX(r) {
		w.Header().Set("HX-Trigger", "linkUpdated")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/dashboard/links/"+id, http.StatusSeeOther)
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

// ConfirmDelete renders the delete confirmation modal for a link.
// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
func (h *LinksHandler) ConfirmDelete(w http.ResponseWriter, r *http.Request) {
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

	data := ConfirmDeleteData{
		Name:      link.Slug,
		DeleteURL: "/dashboard/links/" + id,
		Target:    "#link-" + id,
	}
	renderFragment(w, "confirm_delete", data)
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
