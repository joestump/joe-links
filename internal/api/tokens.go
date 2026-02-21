// Governing: SPEC-0006 REQ "Token Management API", ADR-0009
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// tokensAPIHandler provides REST handlers for API token management.
// Governing: SPEC-0006 REQ "Token Management API" — Bearer token auth only.
type tokensAPIHandler struct {
	tokens auth.TokenStore
}

// registerTokenRoutes registers token management routes on r.
// Governing: SPEC-0006 REQ "Token Management API"
func registerTokenRoutes(r chi.Router, tokens auth.TokenStore) {
	h := &tokensAPIHandler{tokens: tokens}
	r.Get("/tokens", h.List)
	r.Post("/tokens", h.Create)
	r.Delete("/tokens/{id}", h.Revoke)
}

// List returns the caller's tokens without sensitive fields.
// GET /api/v1/tokens
// Governing: SPEC-0006 REQ "Token Management API" — response MUST NOT include token_hash.
//
// @Summary      List tokens
// @Description  Returns all API tokens for the authenticated user. Never includes token_hash.
// @Tags         Tokens
// @Accept       json
// @Produce      json
// @Success      200  {object}  TokenListResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerToken
// @Router       /tokens [get]
func (h *tokensAPIHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	records, err := h.tokens.ListByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "internal_error")
		return
	}

	resp := &TokenListResponse{Tokens: make([]*TokenResponse, 0, len(records))}
	for _, rec := range records {
		item := &TokenResponse{
			ID:        rec.ID,
			Name:      rec.Name,
			CreatedAt: rec.CreatedAt,
		}
		if rec.LastUsedAt.Valid {
			t := rec.LastUsedAt.Time
			item.LastUsedAt = &t
		}
		if rec.ExpiresAt.Valid {
			t := rec.ExpiresAt.Time
			item.ExpiresAt = &t
		}
		resp.Tokens = append(resp.Tokens, item)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Create generates a new token and returns the plaintext once.
// POST /api/v1/tokens
// Governing: SPEC-0006 REQ "Token Management API" — plaintext MUST NOT appear in any subsequent call.
//
// @Summary      Create a token
// @Description  Generates a new API token. The plaintext token is returned only in this response.
// @Tags         Tokens
// @Accept       json
// @Produce      json
// @Param        body  body      CreateTokenRequest   true  "Token to create"
// @Success      201   {object}  TokenCreatedResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     BearerToken
// @Router       /tokens [post]
func (h *tokensAPIHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "bad_request")
		return
	}

	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed", "internal_error")
		return
	}

	rec, err := h.tokens.Create(r.Context(), user.ID, req.Name, hash, req.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token creation failed", "internal_error")
		return
	}

	item := &TokenResponse{
		ID:        rec.ID,
		Name:      rec.Name,
		CreatedAt: rec.CreatedAt,
	}
	if rec.ExpiresAt.Valid {
		t := rec.ExpiresAt.Time
		item.ExpiresAt = &t
	}

	writeJSON(w, http.StatusCreated, TokenCreatedResponse{
		TokenResponse: *item,
		Token:         plaintext,
	})
}

// Revoke soft-deletes a token owned by the current user.
// DELETE /api/v1/tokens/{id}
// Governing: SPEC-0006 REQ "Token Management API" — returns 404 for other users' tokens.
//
// @Summary      Revoke a token
// @Description  Soft-deletes an API token. Returns 404 if the token belongs to another user.
// @Tags         Tokens
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Token ID"
// @Success      204  "No Content"
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerToken
// @Router       /tokens/{id} [delete]
func (h *tokensAPIHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	tokenID := chi.URLParam(r, "id")
	err := h.tokens.Revoke(r.Context(), tokenID, user.ID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "revoke failed", "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
