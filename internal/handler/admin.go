// Governing: SPEC-0004 REQ "Admin Dashboard", ADR-0007
package handler

import (
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
)

// AdminHandler serves admin views.
type AdminHandler struct{}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler() *AdminHandler { return &AdminHandler{} }

// Dashboard renders the admin overview.
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := BasePage{Theme: themeFromRequest(r), User: user}
	render(w, "admin/dashboard.html", data)
}

// Users renders the user management list.
func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := BasePage{Theme: themeFromRequest(r), User: user}
	render(w, "admin/users.html", data)
}

// UpdateRole handles PUT /admin/users/{id}/role.
func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Links renders the admin link list.
func (h *AdminHandler) Links(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := BasePage{Theme: themeFromRequest(r), User: user}
	render(w, "admin/links.html", data)
}
