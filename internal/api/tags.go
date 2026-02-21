// Governing: SPEC-0005 REQ "Tags", ADR-0008
package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// tagsAPIHandler provides REST handlers for tag endpoints.
// Governing: SPEC-0005 REQ "Tags"
type tagsAPIHandler struct {
	tags  *store.TagStore
	links *store.LinkStore
}

// registerTagRoutes registers tag routes on r.
// Governing: SPEC-0005 REQ "Tags"
func registerTagRoutes(r chi.Router, tags *store.TagStore, links *store.LinkStore) {
	h := &tagsAPIHandler{tags: tags, links: links}
	r.Get("/tags", h.List)
	r.Get("/tags/{slug}/links", h.ListLinks)
}

// List returns all tags with link_count >= 1.
// GET /api/v1/tags
// Governing: SPEC-0005 REQ "Tags" — tags with link_count = 0 MUST NOT appear.
func (h *tagsAPIHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	tagsWithCounts, err := h.tags.ListWithCounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "internal_error")
		return
	}

	resp := &TagListResponse{Tags: make([]*TagResponse, 0, len(tagsWithCounts))}
	for _, t := range tagsWithCounts {
		resp.Tags = append(resp.Tags, &TagResponse{
			Slug:      t.Slug,
			Name:      t.Name,
			LinkCount: t.Count,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListLinks returns links tagged with the given slug.
// GET /api/v1/tags/{slug}/links
// Governing: SPEC-0005 REQ "Tags" — admin sees all links; non-admin sees only owned links.
func (h *tagsAPIHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	tagSlug := chi.URLParam(r, "slug")

	// Verify the tag exists.
	_, err := h.tags.GetBySlug(r.Context(), tagSlug)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tag not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "internal_error")
		return
	}

	var links []*store.Link
	if user.IsAdmin() {
		links, err = h.links.ListByTag(r.Context(), tagSlug)
	} else {
		links, err = h.links.ListByOwnerAndTag(r.Context(), user.ID, tagSlug)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "internal_error")
		return
	}

	resp := &LinkListResponse{Links: make([]*LinkResponse, 0, len(links))}
	for _, l := range links {
		resp.Links = append(resp.Links, &LinkResponse{
			ID:          l.ID,
			Slug:        l.Slug,
			URL:         l.URL,
			Title:       l.Title,
			Description: l.Description,
			CreatedAt:   l.CreatedAt,
			UpdatedAt:   l.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
