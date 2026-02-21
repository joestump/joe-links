// Governing: SPEC-0005 REQ "Admin Endpoints", ADR-0008
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

// adminAPIHandler provides REST handlers for admin-only endpoints.
// Governing: SPEC-0005 REQ "Admin Endpoints"
type adminAPIHandler struct {
	users     *store.UserStore
	links     *store.LinkStore
	ownership *store.OwnershipStore
}

// registerAdminRoutes registers admin routes inside a chi Group with role-check middleware.
// Governing: SPEC-0005 REQ "Admin Endpoints" — chi Group MUST enforce role = admin.
func registerAdminRoutes(r chi.Router, users *store.UserStore, links *store.LinkStore, ownership *store.OwnershipStore) {
	h := &adminAPIHandler{users: users, links: links, ownership: ownership}

	r.Route("/admin", func(admin chi.Router) {
		// Governing: SPEC-0005 REQ "Admin Endpoints" — non-admin returns 403 Forbidden.
		admin.Use(requireAdmin)

		admin.Get("/users", h.ListUsers)
		admin.Put("/users/{id}/role", h.UpdateRole)
		admin.Get("/links", h.ListLinks)
	})
}

// requireAdmin is middleware that enforces role = admin on all routes in the group.
// Governing: SPEC-0005 REQ "Admin Endpoints" — WHEN role != admin THEN 403 Forbidden.
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r.Context())
		if user == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
			return
		}
		if user.Role != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListUsers returns all users in the system.
// GET /api/v1/admin/users
// Governing: SPEC-0005 REQ "Admin Endpoints"
//
// @Summary      List all users (admin)
// @Description  Returns all users in the system. Requires admin role.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Success      200  {object}  UserListResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerToken
// @Router       /admin/users [get]
func (h *adminAPIHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	resp := &UserListResponse{Users: make([]*UserResponse, 0, len(users))}
	for _, u := range users {
		resp.Users = append(resp.Users, &UserResponse{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			CreatedAt:   u.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateRole changes a user's role. Accepts only "user" and "admin".
// PUT /api/v1/admin/users/{id}/role
// Governing: SPEC-0005 REQ "Admin Endpoints" — valid roles: "user", "admin".
//
// @Summary      Update user role (admin)
// @Description  Changes a user's role. Valid values: "user", "admin". Requires admin role.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "User ID"
// @Param        body  body      UpdateRoleRequest  true  "New role"
// @Success      200   {object}  UserResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     BearerToken
// @Router       /admin/users/{id}/role [put]
func (h *adminAPIHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	if req.Role != "user" && req.Role != "admin" {
		writeError(w, http.StatusBadRequest, "role must be \"user\" or \"admin\"", "BAD_REQUEST")
		return
	}

	updated, err := h.users.UpdateRole(r.Context(), userID, req.Role)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, &UserResponse{
		ID:          updated.ID,
		Email:       updated.Email,
		DisplayName: updated.DisplayName,
		Role:        updated.Role,
		CreatedAt:   updated.CreatedAt,
	})
}

// ListLinks returns all links system-wide (admin-only explicit route).
// GET /api/v1/admin/links
// Governing: SPEC-0005 REQ "Admin Endpoints"
//
// @Summary      List all links (admin)
// @Description  Returns all links system-wide with owners and tags. Requires admin role.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Success      200  {object}  LinkListResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerToken
// @Router       /admin/links [get]
func (h *adminAPIHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := h.links.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	resp := &LinkListResponse{Links: make([]*LinkResponse, 0, len(links))}
	for _, l := range links {
		// Build owner list for each link.
		owners, err := h.ownership.ListOwnerUsers(l.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		ownerResponses := make([]OwnerResponse, 0, len(owners))
		for _, o := range owners {
			ownerResponses = append(ownerResponses, OwnerResponse{
				ID:        o.ID,
				Email:     o.Email,
				IsPrimary: o.IsPrimary,
			})
		}

		// Build tag list for each link.
		tags, err := h.links.ListTags(r.Context(), l.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
			return
		}
		tagNames := make([]string, 0, len(tags))
		for _, t := range tags {
			tagNames = append(tagNames, t.Name)
		}

		resp.Links = append(resp.Links, &LinkResponse{
			ID:          l.ID,
			Slug:        l.Slug,
			URL:         l.URL,
			Title:       l.Title,
			Description: l.Description,
			Tags:        tagNames,
			Owners:      ownerResponses,
			CreatedAt:   l.CreatedAt,
			UpdatedAt:   l.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
