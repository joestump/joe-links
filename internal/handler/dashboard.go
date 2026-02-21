// Governing: SPEC-0001 REQ "Short Link Management", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
package handler

import (
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// DashboardPage is the template data for the dashboard view.
type DashboardPage struct {
	BasePage
	User  *store.User
	Links []*store.Link
	Flash *Flash
}

// DashboardHandler serves the authenticated link management dashboard.
type DashboardHandler struct {
	links *store.LinkStore
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(ls *store.LinkStore) *DashboardHandler {
	return &DashboardHandler{links: ls}
}

// Show renders the dashboard with the user's links (or all links for admins).
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	var links []*store.Link
	var err error
	if user.IsAdmin() {
		links, err = h.links.ListAll(r.Context())
	} else {
		links, err = h.links.ListByOwner(r.Context(), user.ID)
	}
	if err != nil {
		http.Error(w, "could not load links", http.StatusInternalServerError)
		return
	}

	data := DashboardPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Links: links}
	if isHTMX(r) {
		renderFragment(w, "content", data)
		return
	}
	render(w, "dashboard.html", data)
}
