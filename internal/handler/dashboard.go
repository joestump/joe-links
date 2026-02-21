// Governing: SPEC-0001 REQ "Short Link Management", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
// Governing: SPEC-0004 REQ "User Dashboard"
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
	Tags  []*store.Tag
	Query string // current search query
	Tag   string // current tag filter slug
	Flash *Flash
}

// DashboardHandler serves the authenticated link management dashboard.
type DashboardHandler struct {
	links *store.LinkStore
	tags  *store.TagStore
}

// NewDashboardHandler creates a new DashboardHandler.
// Governing: SPEC-0004 REQ "User Dashboard"
func NewDashboardHandler(ls *store.LinkStore, ts *store.TagStore) *DashboardHandler {
	return &DashboardHandler{links: ls, tags: ts}
}

// Show renders the dashboard with the user's links (or all links for admins).
// Supports ?q= for search and ?tag= for tag filtering via HTMX.
// Governing: SPEC-0004 REQ "User Dashboard"
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	query := r.URL.Query().Get("q")
	tagSlug := r.URL.Query().Get("tag")

	var links []*store.Link
	var err error

	switch {
	case tagSlug != "":
		// Tag filter takes precedence
		if user.IsAdmin() {
			links, err = h.links.ListByTag(r.Context(), tagSlug)
		} else {
			links, err = h.links.ListByOwnerAndTag(r.Context(), user.ID, tagSlug)
		}
	case query != "":
		// Search filter
		if user.IsAdmin() {
			links, err = h.links.SearchAll(r.Context(), query)
		} else {
			links, err = h.links.SearchByOwner(r.Context(), user.ID, query)
		}
	default:
		// No filters
		if user.IsAdmin() {
			links, err = h.links.ListAll(r.Context())
		} else {
			links, err = h.links.ListByOwner(r.Context(), user.ID)
		}
	}
	if err != nil {
		http.Error(w, "could not load links", http.StatusInternalServerError)
		return
	}

	// Load all tags for the tag filter chips
	allTags, _ := h.tags.ListAll(r.Context())

	data := DashboardPage{
		BasePage: BasePage{Theme: themeFromRequest(r), User: user},
		User:     user,
		Links:    links,
		Tags:     allTags,
		Query:    query,
		Tag:      tagSlug,
	}

	if isHTMX(r) {
		renderFragment(w, "link_list", data)
		return
	}
	render(w, "dashboard.html", data)
}
