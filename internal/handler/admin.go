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

// UserRowData wraps a user row with the current admin's ID for conditional rendering.
// Governing: SPEC-0011 REQ "Admin User Deletion with Link Handling" — hide delete for self
type UserRowData struct {
	*store.User
	CurrentUserID string
}

// AdminUsersPage is the template data for the user management list.
type AdminUsersPage struct {
	BasePage
	Rows []UserRowData
}

// AdminLinksPage is the template data for the admin link list.
type AdminLinksPage struct {
	BasePage
	Links []*store.Link
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
	rows := make([]UserRowData, len(allUsers))
	for i, u := range allUsers {
		rows[i] = UserRowData{User: u, CurrentUserID: user.ID}
	}
	data := AdminUsersPage{
		BasePage: newBasePage(r, user),
		Rows:     rows,
	}
	render(w, "admin/users.html", data)
}

// UpdateRole handles PUT /admin/users/{id}/role — updates role and returns updated row fragment.
// Governing: SPEC-0004 REQ "Admin Dashboard" — inline role toggle via HTMX
func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.UserFromContext(r.Context())
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
	row := UserRowData{User: target, CurrentUserID: currentUser.ID}
	w.Header().Set("Content-Type", "text/html")
	renderPageFragment(w, "admin/users.html", "user_row", row)
}

// Links renders the admin link list (all links across all users).
// Governing: SPEC-0004 REQ "Admin Dashboard"
func (h *AdminHandler) Links(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	allLinks, _ := h.links.ListAll(r.Context())
	data := AdminLinksPage{
		BasePage: newBasePage(r, user),
		Links:    allLinks,
	}
	render(w, "admin/links.html", data)
}

// UserDeleteModalData holds template data for the user deletion confirmation modal.
// Governing: SPEC-0011 REQ "Admin User Deletion with Link Handling"
type UserDeleteModalData struct {
	UserID      string
	DisplayName string
	Email       string
	LinkCount   int
	DeleteURL   string
}

// ConfirmDeleteUser renders the custom user deletion modal with link count and disposition options.
// Governing: SPEC-0011 REQ "Admin User Deletion with Link Handling"
func (h *AdminHandler) ConfirmDeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Guard: admin cannot delete themselves
	if id == currentUser.ID {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}

	target, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	linkCount, err := h.users.CountPrimaryLinks(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to count links", http.StatusInternalServerError)
		return
	}

	data := UserDeleteModalData{
		UserID:      target.ID,
		DisplayName: target.DisplayName,
		Email:       target.Email,
		LinkCount:   linkCount,
		DeleteURL:   "/admin/users/" + id,
	}
	renderFragment(w, "admin_user_delete_modal", data)
}

// DeleteUser handles DELETE /admin/users/{id} — deletes a user with link disposition.
// Governing: SPEC-0011 REQ "Admin User Deletion Endpoint", ADR-0005, ADR-0007
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Guard: admin cannot delete themselves
	if id == currentUser.ID {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	linkAction := r.FormValue("link_action")

	// Check how many links the target owns
	linkCount, err := h.users.CountPrimaryLinks(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to count links", http.StatusInternalServerError)
		return
	}

	// Require link_action when user owns links
	if linkCount > 0 && linkAction != "reassign" && linkAction != "delete" {
		http.Error(w, "link_action required (reassign or delete)", http.StatusBadRequest)
		return
	}

	// Default to "delete" when user has no links (link_action is irrelevant)
	if linkCount == 0 {
		linkAction = "delete"
	}

	if err := h.users.DeleteUserWithLinks(r.Context(), id, currentUser.ID, linkAction); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	// Return empty response so HTMX removes the row, plus OOB toast
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<div id="toast-area" hx-swap-oob="innerHTML:#toast-area"><div class="alert alert-success"><span>User deleted.</span></div></div>`))
}
