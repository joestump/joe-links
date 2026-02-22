// Governing: SPEC-0004 REQ "Admin Dashboard", ADR-0007
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// AdminHandler serves admin views.
type AdminHandler struct {
	links    *store.LinkStore
	users    *store.UserStore
	keywords *store.KeywordStore
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(ls *store.LinkStore, us *store.UserStore, ks *store.KeywordStore) *AdminHandler {
	return &AdminHandler{links: ls, users: us, keywords: ks}
}

// AdminDashboardPage is the template data for the admin overview.
type AdminDashboardPage struct {
	BasePage
	UserCount    int
	LinkCount    int
	KeywordCount int
}

// AdminUsersPage is the template data for the user management list.
type AdminUsersPage struct {
	BasePage
	Users []*store.User
}

// AdminLinksPage is the template data for the admin link list.
// Governing: SPEC-0011 REQ "Admin Links Screen"
type AdminLinksPage struct {
	BasePage
	Links []*store.AdminLink
	Query string
}

// Dashboard renders the admin overview with summary stats.
// Governing: SPEC-0004 REQ "Admin Dashboard"
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	allUsers, _ := h.users.ListAll(r.Context())
	allLinks, _ := h.links.ListAll(r.Context())
	allKeywords, _ := h.keywords.List(r.Context())
	data := AdminDashboardPage{
		BasePage:     newBasePage(r, user),
		UserCount:    len(allUsers),
		LinkCount:    len(allLinks),
		KeywordCount: len(allKeywords),
	}
	render(w, "admin/dashboard.html", data)
}

// Users renders the user management list.
// Governing: SPEC-0004 REQ "Admin Dashboard"
func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	allUsers, _ := h.users.ListAll(r.Context())
	data := AdminUsersPage{
		BasePage: newBasePage(r, user),
		Users:    allUsers,
	}
	render(w, "admin/users.html", data)
}

// UpdateRole handles PUT /admin/users/{id}/role — updates role and returns updated row fragment.
// Governing: SPEC-0004 REQ "Admin Dashboard" — inline role toggle via HTMX
func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	role := r.FormValue("role")
	if role != "admin" && role != "user" {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	target, err := h.users.UpdateRole(r.Context(), id, role)
	if err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	renderPageFragment(w, "admin/users.html", "user_row", target)
}

// Links renders the admin link list (all links across all users).
// Supports HTMX search via ?q= query parameter with debounce.
// Governing: SPEC-0011 REQ "Admin Links Screen", ADR-0007
func (h *AdminHandler) Links(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	q := r.URL.Query().Get("q")
	allLinks, _ := h.links.ListAllAdmin(r.Context(), q)
	data := AdminLinksPage{
		BasePage: newBasePage(r, user),
		Links:    allLinks,
		Query:    q,
	}
	if isHTMX(r) {
		renderPageFragment(w, "admin/links.html", "admin_link_list", data)
		return
	}
	render(w, "admin/links.html", data)
}

// EditLinkRow returns an editable <tr> fragment for inline link editing.
// Governing: SPEC-0011 REQ "Admin Inline Link Editing", ADR-0007
func (h *AdminHandler) EditLinkRow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.links.GetAdminLink(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderPageFragment(w, "admin/links.html", "admin_link_edit_row", link)
}

// UpdateLink handles PUT /admin/links/{id} — updates url, title, description and returns the read-only row.
// Governing: SPEC-0011 REQ "Admin Link Deletion Endpoint", ADR-0005
func (h *AdminHandler) UpdateLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	url := r.FormValue("url")
	title := r.FormValue("title")
	description := r.FormValue("description")

	_, err := h.links.Update(r.Context(), id, url, title, description)
	if err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	link, err := h.links.GetAdminLink(r.Context(), id)
	if err != nil {
		http.Error(w, "fetch failed", http.StatusInternalServerError)
		return
	}
	renderPageFragment(w, "admin/links.html", "admin_link_row", link)
}

// DeleteLink handles DELETE /admin/links/{id} — removes the link and returns an OOB toast.
// Governing: SPEC-0011 REQ "Admin Link Deletion Endpoint", ADR-0005
func (h *AdminHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.links.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<div id="toast-area" hx-swap-oob="innerHTML:#toast-area"><div class="alert alert-success"><span>Link deleted.</span></div></div>`))
}

// LinkRow returns the read-only <tr> fragment for a single admin link row.
// Used by the Cancel button during inline editing to restore the original row.
// Governing: SPEC-0011 REQ "Admin Inline Link Editing"
func (h *AdminHandler) LinkRow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.links.GetAdminLink(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderPageFragment(w, "admin/links.html", "admin_link_row", link)
}

// ConfirmDeleteLink renders the delete confirmation modal for a link.
// Governing: SPEC-0011 REQ "Admin Link Deletion", SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
func (h *AdminHandler) ConfirmDeleteLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data := ConfirmDeleteData{
		Name:      link.Slug,
		DeleteURL: "/admin/links/" + id,
		Target:    "#admin-link-" + id,
	}
	renderFragment(w, "confirm_delete", data)
}

// ConfirmDeleteUser renders the delete confirmation modal for a user.
// Governing: SPEC-0013 REQ "DaisyUI Delete Confirmation Modal"
func (h *AdminHandler) ConfirmDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	target, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := ConfirmDeleteData{
		Name:      target.DisplayName,
		DeleteURL: "/admin/users/" + id,
		Target:    "#user-" + id,
	}
	renderFragment(w, "confirm_delete", data)
}
