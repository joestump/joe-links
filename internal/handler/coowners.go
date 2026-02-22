// Governing: SPEC-0004 REQ "Co-Owner Management", "Link Detail View", ADR-0007
// Governing: SPEC-0010 REQ "Share Management Panel on Link Detail", "Link Share Management Endpoints"
package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// AddOwner handles POST /dashboard/links/{id}/owners.
// Accepts form field "email" to add a co-owner by email address.
// Governing: SPEC-0004 REQ "Co-Owner Management"
func (h *LinksHandler) AddOwner(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	if email == "" {
		h.renderOwnersError(w, r, link, user, "Email is required.")
		return
	}

	target, err := h.users.GetByEmail(r.Context(), email)
	if err != nil {
		h.renderOwnersError(w, r, link, user, "No user found with that email.")
		return
	}

	if err := h.links.AddOwner(r.Context(), link.ID, target.ID); err != nil {
		if errors.Is(err, store.ErrDuplicateOwner) {
			h.renderOwnersError(w, r, link, user, "User is already a co-owner.")
			return
		}
		h.renderOwnersError(w, r, link, user, "Could not add co-owner.")
		return
	}

	h.renderOwnersFragment(w, link)
}

// RemoveOwner handles DELETE /dashboard/links/{id}/owners/{uid}.
// Governing: SPEC-0004 REQ "Co-Owner Management"
func (h *LinksHandler) RemoveOwner(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	uid := chi.URLParam(r, "uid")
	if err := h.links.RemoveOwner(r.Context(), link.ID, uid); err != nil {
		if errors.Is(err, store.ErrPrimaryOwnerImmutable) {
			http.Error(w, "Cannot remove primary owner", http.StatusBadRequest)
			return
		}
		http.Error(w, "Could not remove co-owner", http.StatusInternalServerError)
		return
	}

	h.renderOwnersFragment(w, link)
}

// Detail handles GET /dashboard/links/{id}.
// Governing: SPEC-0004 REQ "Link Detail View"
func (h *LinksHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	tags, _ := h.links.ListTags(r.Context(), link.ID)
	owners, _ := h.owns.ListOwnerUsers(link.ID)

	// Governing: SPEC-0010 REQ "Share Management Panel on Link Detail"
	var shares []ShareUser
	if link.Visibility == "secure" {
		shareRecords, _ := h.links.ListShares(r.Context(), link.ID)
		for _, sr := range shareRecords {
			u, err := h.users.GetByID(r.Context(), sr.UserID)
			if err != nil {
				continue
			}
			shares = append(shares, ShareUser{
				UserID:      u.ID,
				DisplayName: u.DisplayName,
				Email:       u.Email,
			})
		}
	}

	data := LinkDetailPage{
		BasePage: newBasePage(r, user),
		User:     user,
		Link:     link,
		Tags:     tags,
		Owners:   owners,
		Shares:   shares,
	}
	if isHTMX(r) {
		renderPageFragment(w, "links/detail.html", "content", data)
		return
	}
	render(w, "links/detail.html", data)
}

// ValidateSlug handles GET /dashboard/links/validate-slug?slug=...
// Governing: SPEC-0004 REQ "New Link Form" — live slug validation
func (h *LinksHandler) ValidateSlug(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	w.Header().Set("Content-Type", "text/html")
	if slug == "" {
		w.Write([]byte(""))
		return
	}
	if err := store.ValidateSlugFormat(slug); err != nil {
		w.Write([]byte(`<span class="text-error text-xs">` + err.Error() + `</span>`))
		return
	}
	if _, err := h.links.GetBySlug(r.Context(), slug); err == nil {
		w.Write([]byte(`<span class="text-error text-xs">Slug already taken</span>`))
		return
	}
	w.Write([]byte(`<span class="text-success text-xs">Available!</span>`))
}

// renderOwnersFragment re-renders the owners list for HTMX swap.
func (h *LinksHandler) renderOwnersFragment(w http.ResponseWriter, link *store.Link) {
	owners, _ := h.owns.ListOwnerUsers(link.ID)
	w.Header().Set("Content-Type", "text/html")
	renderFragment(w, "owners_list", &ownersFragmentData{Link: link, Owners: owners})
}

// renderOwnersError renders owners fragment with an error message.
func (h *LinksHandler) renderOwnersError(w http.ResponseWriter, r *http.Request, link *store.Link, user *store.User, errMsg string) {
	owners, _ := h.owns.ListOwnerUsers(link.ID)
	w.Header().Set("Content-Type", "text/html")
	renderFragment(w, "owners_list", &ownersFragmentData{Link: link, Owners: owners, Error: errMsg})
}

type ownersFragmentData struct {
	Link   *store.Link
	Owners []*store.OwnerInfo
	Error  string
}

// AddShare handles POST /dashboard/links/{id}/shares.
// Accepts form field "email" to grant a user access to a secure link.
// Governing: SPEC-0010 REQ "Link Share Management Endpoints"
func (h *LinksHandler) AddShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Governing: SPEC-0010 REQ "Link Share Management Endpoints" — only owners, co-owners, and admins
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	if email == "" {
		h.renderSharesError(w, r, link, "Email is required.")
		return
	}

	target, err := h.users.GetByEmail(r.Context(), email)
	if err != nil {
		h.renderSharesError(w, r, link, "No user found with that email.")
		return
	}

	if err := h.links.AddShare(r.Context(), link.ID, target.ID, user.ID); err != nil {
		h.renderSharesError(w, r, link, "Could not add user. They may already have access.")
		return
	}

	h.renderSharesFragment(w, r, link)
}

// RemoveShare handles DELETE /dashboard/links/{id}/shares/{uid}.
// Governing: SPEC-0010 REQ "Link Share Management Endpoints"
func (h *LinksHandler) RemoveShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Governing: SPEC-0010 REQ "Link Share Management Endpoints" — only owners, co-owners, and admins
	allowed, err := store.IsOwnerOrAdmin(h.owns, link.ID, user.ID, user.Role)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	uid := chi.URLParam(r, "uid")
	if err := h.links.RemoveShare(r.Context(), link.ID, uid); err != nil {
		http.Error(w, "Could not remove user", http.StatusInternalServerError)
		return
	}

	h.renderSharesFragment(w, r, link)
}

// sharesFragmentData holds template data for the shares panel HTMX fragment.
type sharesFragmentData struct {
	Link   *store.Link
	Shares []ShareUser
	Error  string
}

// renderSharesFragment re-renders the shares panel for HTMX swap.
func (h *LinksHandler) renderSharesFragment(w http.ResponseWriter, r *http.Request, link *store.Link) {
	shares := h.loadShares(r, link)
	renderFragment(w, "shares_panel", &sharesFragmentData{Link: link, Shares: shares})
}

// renderSharesError renders shares panel with an inline validation error.
func (h *LinksHandler) renderSharesError(w http.ResponseWriter, r *http.Request, link *store.Link, errMsg string) {
	shares := h.loadShares(r, link)
	renderFragment(w, "shares_panel", &sharesFragmentData{Link: link, Shares: shares, Error: errMsg})
}

// loadShares resolves share records to ShareUser display objects.
func (h *LinksHandler) loadShares(r *http.Request, link *store.Link) []ShareUser {
	var shares []ShareUser
	shareRecords, _ := h.links.ListShares(r.Context(), link.ID)
	for _, sr := range shareRecords {
		u, err := h.users.GetByID(r.Context(), sr.UserID)
		if err != nil {
			continue
		}
		shares = append(shares, ShareUser{
			UserID:      u.ID,
			DisplayName: u.DisplayName,
			Email:       u.Email,
		})
	}
	return shares
}
