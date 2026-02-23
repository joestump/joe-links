// Governing: SPEC-0012 REQ "User Profile Page (GET /u/{display_name_slug})"
package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

const profilePageSize = 25

// ProfilePage is the template data for the user profile page.
// Governing: SPEC-0012 REQ "User Profile Page (GET /u/{display_name_slug})"
type ProfilePage struct {
	BasePage
	ProfileUser *store.User
	Links       []store.PublicLink
	Page        int
	TotalPages  int
	TotalLinks  int
	PrevPage    int
	NextPage    int
}

// ProfileHandler provides HTTP handlers for public user profile pages.
type ProfileHandler struct {
	users *store.UserStore
	links *store.LinkStore
}

// NewProfileHandler creates a new ProfileHandler.
func NewProfileHandler(us *store.UserStore, ls *store.LinkStore) *ProfileHandler {
	return &ProfileHandler{users: us, links: ls}
}

// Show renders the public user profile page at GET /u/{displayNameSlug}.
// Governing: SPEC-0012 REQ "User Profile Page (GET /u/{display_name_slug})"
func (h *ProfileHandler) Show(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "displayNameSlug")

	profileUser, err := h.users.GetByDisplayNameSlug(r.Context(), slug)
	if err != nil {
		if err == store.ErrNotFound {
			viewer := auth.UserFromContext(r.Context())
			w.WriteHeader(http.StatusNotFound)
			data := notFoundPage{BasePage: newBasePage(r, viewer), User: viewer, Slug: "u/" + slug}
			if isHTMX(r) {
				renderPageFragment(w, "404.html", "content", data)
				return
			}
			render(w, "404.html", data)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	links, total, err := h.links.ListPublicByOwner(r.Context(), profileUser.ID, page, profilePageSize)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + profilePageSize - 1) / profilePageSize
	if totalPages < 1 {
		totalPages = 1
	}

	viewer := auth.UserFromContext(r.Context())
	data := ProfilePage{
		BasePage:    newBasePage(r, viewer),
		ProfileUser: profileUser,
		Links:       links,
		Page:        page,
		TotalPages:  totalPages,
		TotalLinks:  total,
		PrevPage:    page - 1,
		NextPage:    page + 1,
	}

	if isHTMX(r) {
		renderPageFragment(w, "profile.html", "content", data)
		return
	}
	render(w, "profile.html", data)
}
