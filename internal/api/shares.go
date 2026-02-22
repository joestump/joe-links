// Governing: SPEC-0010 REQ "Link Share Management API Endpoints", SPEC-0005 REQ "API Router Mounting"
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// sharesAPIHandler provides REST handlers for link share management.
// Governing: SPEC-0010 REQ "Link Share Management API Endpoints"
type sharesAPIHandler struct {
	links     *store.LinkStore
	ownership *store.OwnershipStore
	users     *store.UserStore
}

// registerShareRoutes registers share management routes on r.
// Governing: SPEC-0010 REQ "Link Share Management API Endpoints"
func registerShareRoutes(r chi.Router, links *store.LinkStore, ownership *store.OwnershipStore, users *store.UserStore) {
	h := &sharesAPIHandler{links: links, ownership: ownership, users: users}
	r.Get("/links/{id}/shares", h.List)
	r.Post("/links/{id}/shares", h.Add)
	r.Delete("/links/{id}/shares/{uid}", h.Remove)
}

// List returns all users with share access to a link.
// GET /api/v1/links/{id}/shares
// Governing: SPEC-0010 REQ "Link Share Management API Endpoints"
//
// @Summary      List link shares
// @Description  Returns all users who have been shared access to a link. Only owners and admins may access.
// @Tags         Shares
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Link ID"
// @Success      200  {array}   ShareResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerToken
// @Router       /links/{id}/shares [get]
func (h *sharesAPIHandler) List(w http.ResponseWriter, r *http.Request) {
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
		isOwner, err := h.ownership.IsOwner(link.ID, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
	}

	shares, err := h.links.ListShares(r.Context(), link.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	resp := make([]ShareResponse, 0, len(shares))
	for _, s := range shares {
		u, err := h.users.GetByID(r.Context(), s.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		resp = append(resp, ShareResponse{
			LinkID:      s.LinkID,
			UserID:      s.UserID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			SharedBy:    s.SharedBy,
			CreatedAt:   s.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// Add shares a link with a user by email.
// POST /api/v1/links/{id}/shares
// Governing: SPEC-0010 REQ "Link Share Management API Endpoints"
//
// @Summary      Add a share
// @Description  Shares a link with a user by email address. Only owners and admins may share.
// @Tags         Shares
// @Accept       json
// @Produce      json
// @Param        id    path      string           true  "Link ID"
// @Param        body  body      AddShareRequest  true  "User to share with"
// @Success      201   {object}  ShareResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     BearerToken
// @Router       /links/{id}/shares [post]
func (h *sharesAPIHandler) Add(w http.ResponseWriter, r *http.Request) {
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
		isOwner, err := h.ownership.IsOwner(link.ID, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
	}

	var req AddShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required", "BAD_REQUEST")
		return
	}

	targetUser, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	// Check if already shared.
	hasShare, err := h.links.HasShare(r.Context(), link.ID, targetUser.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	if hasShare {
		writeError(w, http.StatusConflict, "link is already shared with this user", "DUPLICATE_SHARE")
		return
	}

	if err := h.links.AddShare(r.Context(), link.ID, targetUser.ID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	// Fetch the created share record for the response.
	shares, err := h.links.ListShares(r.Context(), link.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	for _, s := range shares {
		if s.UserID == targetUser.ID {
			writeJSON(w, http.StatusCreated, ShareResponse{
				LinkID:      s.LinkID,
				UserID:      s.UserID,
				Email:       targetUser.Email,
				DisplayName: targetUser.DisplayName,
				SharedBy:    s.SharedBy,
				CreatedAt:   s.CreatedAt,
			})
			return
		}
	}

	writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
}

// Remove revokes a user's share access to a link.
// DELETE /api/v1/links/{id}/shares/{uid}
// Governing: SPEC-0010 REQ "Link Share Management API Endpoints"
//
// @Summary      Remove a share
// @Description  Removes a user's share access to a link. Only owners and admins may remove shares.
// @Tags         Shares
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Link ID"
// @Param        uid  path      string  true  "User ID of the share to remove"
// @Success      204  "No Content"
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerToken
// @Router       /links/{id}/shares/{uid} [delete]
func (h *sharesAPIHandler) Remove(w http.ResponseWriter, r *http.Request) {
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
		isOwner, err := h.ownership.IsOwner(link.ID, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
	}

	shareUID := chi.URLParam(r, "uid")

	// Verify the share exists before deleting.
	hasShare, err := h.links.HasShare(r.Context(), link.ID, shareUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}
	if !hasShare {
		writeError(w, http.StatusNotFound, "share not found", "NOT_FOUND")
		return
	}

	if err := h.links.RemoveShare(r.Context(), link.ID, shareUID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
