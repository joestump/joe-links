// Governing: SPEC-0016 REQ "REST API Stats Endpoint", REQ "REST API Clicks Endpoint", ADR-0016, ADR-0008, ADR-0009
package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

type statsAPIHandler struct {
	links  *store.LinkStore
	clicks *store.ClickStore
	owns   *store.OwnershipStore
}

func newStatsAPIHandler(ls *store.LinkStore, cs *store.ClickStore, os *store.OwnershipStore) *statsAPIHandler {
	return &statsAPIHandler{links: ls, clicks: cs, owns: os}
}

// statsResponse is the JSON shape for GET /api/v1/links/{id}/stats.
type statsResponse struct {
	LinkID  string `json:"link_id"`
	Total   int64  `json:"total"`
	Last7d  int64  `json:"last_7d"`
	Last30d int64  `json:"last_30d"`
}

// clickResponse is one entry in the clicks list.
type clickResponse struct {
	ClickedAt time.Time     `json:"clicked_at"`
	Referrer  *string       `json:"referrer"`
	User      *clickUserRef `json:"user"`
}

type clickUserRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// clickListResponse is the JSON shape for GET /api/v1/links/{id}/clicks.
type clickListResponse struct {
	Clicks     []clickResponse `json:"clicks"`
	NextCursor *string         `json:"next_cursor"`
}

// GetStats returns aggregate click stats for a link.
// GET /api/v1/links/{id}/stats
// Governing: SPEC-0016 REQ "REST API Stats Endpoint", ADR-0016
func (h *statsAPIHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	linkID := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), linkID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	if user.Role != "admin" {
		isOwner, err := h.owns.IsOwner(link.ID, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
	}

	stats, err := h.clicks.GetClickStats(r.Context(), link.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, statsResponse{
		LinkID:  link.ID,
		Total:   stats.Total,
		Last7d:  stats.Last7d,
		Last30d: stats.Last30d,
	})
}

// ListClicks returns paginated click events for a link.
// GET /api/v1/links/{id}/clicks
// Governing: SPEC-0016 REQ "REST API Clicks Endpoint", ADR-0016
func (h *statsAPIHandler) ListClicks(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	linkID := chi.URLParam(r, "id")
	link, err := h.links.GetByID(r.Context(), linkID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	if user.Role != "admin" {
		isOwner, err := h.owns.IsOwner(link.ID, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
	}

	// Parse limit (default 50, max 200).
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	// Parse before cursor (ISO 8601 / RFC 3339 timestamp).
	var before time.Time
	if v := r.URL.Query().Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid before timestamp, expected RFC 3339", "BAD_REQUEST")
			return
		}
		before = t
	}

	// Fetch limit+1 to detect next page.
	rows, err := h.clicks.ListRecentClicksBefore(r.Context(), link.ID, before, limit+1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	var nextCursor *string
	if len(rows) > limit {
		cursor := rows[limit-1].ClickedAt.Format(time.RFC3339Nano)
		nextCursor = &cursor
		rows = rows[:limit]
	}

	clicks := make([]clickResponse, 0, len(rows))
	for _, rc := range rows {
		cr := clickResponse{
			ClickedAt: rc.ClickedAt,
		}
		if rc.Referrer != "" {
			ref := rc.Referrer
			cr.Referrer = &ref
		}
		if rc.UserID != "" {
			cr.User = &clickUserRef{
				ID:          rc.UserID,
				DisplayName: rc.DisplayName,
			}
		}
		clicks = append(clicks, cr)
	}

	writeJSON(w, http.StatusOK, clickListResponse{
		Clicks:     clicks,
		NextCursor: nextCursor,
	})
}
