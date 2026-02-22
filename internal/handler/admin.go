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
