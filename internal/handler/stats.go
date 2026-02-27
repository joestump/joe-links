// Governing: SPEC-0016 REQ "Link Stats Dashboard Page", ADR-0016
package handler

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// StatsPage is the template data for the link analytics view.
// Governing: SPEC-0016 REQ "Link Stats Dashboard Page", ADR-0016
type StatsPage struct {
	BasePage
	User         *store.User
	Link         *store.Link
	Stats        store.ClickStats
	RecentClicks []store.RecentClick
}

// StatsHandler serves the per-link analytics page.
type StatsHandler struct {
	links  *store.LinkStore
	clicks *store.ClickStore
	owns   *store.OwnershipStore
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(ls *store.LinkStore, cs *store.ClickStore, os *store.OwnershipStore) *StatsHandler {
	return &StatsHandler{links: ls, clicks: cs, owns: os}
}

// Show renders the stats page for a single link.
// Governing: SPEC-0016 REQ "Link Stats Dashboard Page", ADR-0016
func (h *StatsHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}

	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check ownership: user must be owner/co-owner or admin
	if !user.IsAdmin() {
		isOwner, _ := h.owns.IsOwner(link.ID, user.ID)
		if !isOwner {
			w.WriteHeader(http.StatusForbidden)
			render(w, "403.html", newBasePage(r, user))
			return
		}
	}

	stats, err := h.clicks.GetClickStats(r.Context(), link.ID)
	if err != nil {
		http.Error(w, "could not load stats", http.StatusInternalServerError)
		return
	}

	recent, err := h.clicks.ListRecentClicks(r.Context(), link.ID, 50)
	if err != nil {
		http.Error(w, "could not load recent clicks", http.StatusInternalServerError)
		return
	}

	data := StatsPage{
		BasePage:     newBasePage(r, user),
		User:         user,
		Link:         link,
		Stats:        stats,
		RecentClicks: recent,
	}

	if isHTMX(r) {
		renderPageFragment(w, "links/stats.html", "content", data)
		return
	}
	render(w, "links/stats.html", data)
}
