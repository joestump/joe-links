// Governing: SPEC-0006 REQ "Token Management Web UI", ADR-0009
package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// TokensPage is the template data for the token management settings page.
type TokensPage struct {
	BasePage
	User     *store.User
	Tokens   []*auth.TokenRecord
	NewToken string // plaintext shown once after creation; empty otherwise
	Flash    *Flash
	Error    string
}

// TokensHandler provides web UI handlers for token management.
type TokensHandler struct {
	tokens auth.TokenStore
}

// NewTokensHandler creates a new TokensHandler.
func NewTokensHandler(ts auth.TokenStore) *TokensHandler {
	return &TokensHandler{tokens: ts}
}

// Index renders the token management page with the user's active tokens.
// GET /dashboard/settings/tokens
// Governing: SPEC-0006 REQ "Token Management Web UI"
func (h *TokensHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	records, err := h.tokens.ListByUser(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "could not load tokens", http.StatusInternalServerError)
		return
	}

	data := TokensPage{
		BasePage: newBasePage(r, user),
		User:     user,
		Tokens:   records,
	}

	if isHTMX(r) {
		renderFragment(w, "token_list", data)
		return
	}
	render(w, "tokens.html", data)
}

// Create processes the token creation form and shows the plaintext once.
// POST /dashboard/settings/tokens
// Governing: SPEC-0006 REQ "Token Management Web UI" — one-time reveal of plaintext.
func (h *TokensHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.renderWithError(w, r, user, "Token name is required.")
		return
	}

	var expiresAt *time.Time
	if exp := r.FormValue("expires_in"); exp != "" {
		d, err := time.ParseDuration(exp)
		if err != nil {
			h.renderWithError(w, r, user, "Invalid expiry duration.")
			return
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		h.renderWithError(w, r, user, "Failed to generate token.")
		return
	}

	_, err = h.tokens.Create(r.Context(), user.ID, name, hash, expiresAt)
	if err != nil {
		h.renderWithError(w, r, user, "Failed to create token.")
		return
	}

	records, _ := h.tokens.ListByUser(r.Context(), user.ID)

	data := TokensPage{
		BasePage: newBasePage(r, user),
		User:     user,
		Tokens:   records,
		NewToken: plaintext,
		Flash:    &Flash{Type: "success", Message: "Token created. Copy it now — it will not be shown again."},
	}

	if isHTMX(r) {
		renderFragment(w, "token_list", data)
		return
	}
	render(w, "tokens.html", data)
}

// Revoke soft-deletes a token owned by the current user.
// DELETE /dashboard/settings/tokens/{id}
// Governing: SPEC-0006 REQ "Token Management Web UI" — revocation with confirmation.
func (h *TokensHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	tokenID := chi.URLParam(r, "id")

	err := h.tokens.Revoke(r.Context(), tokenID, user.ID)
	if err == store.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "revoke failed", http.StatusInternalServerError)
		return
	}

	records, _ := h.tokens.ListByUser(r.Context(), user.ID)

	data := TokensPage{
		BasePage: newBasePage(r, user),
		User:     user,
		Tokens:   records,
		Flash:    &Flash{Type: "success", Message: "Token revoked."},
	}

	if isHTMX(r) {
		renderFragment(w, "token_list", data)
		return
	}
	render(w, "tokens.html", data)
}

func (h *TokensHandler) renderWithError(w http.ResponseWriter, r *http.Request, user *store.User, errMsg string) {
	records, _ := h.tokens.ListByUser(r.Context(), user.ID)
	data := TokensPage{
		BasePage: newBasePage(r, user),
		User:     user,
		Tokens:   records,
		Error:    errMsg,
	}
	if isHTMX(r) {
		renderFragment(w, "token_list", data)
		return
	}
	render(w, "tokens.html", data)
}
