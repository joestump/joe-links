// Governing: SPEC-0001 REQ "OIDC-Only Authentication", ADR-0003
package auth

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/joestump/joe-links/internal/store"
)

const (
	cookieState        = "__auth_state"
	cookieCodeVerifier = "__auth_pkce"
	cookieRedirect     = "__auth_redirect"
)

// Handlers provides HTTP handlers for the OIDC authentication flow.
type Handlers struct {
	provider   *Provider
	sessions   *scs.SessionManager
	users      *store.UserStore
	adminEmail string
}

// NewHandlers creates a new Handlers with the given dependencies.
func NewHandlers(p *Provider, sm *scs.SessionManager, us *store.UserStore, adminEmail string) *Handlers {
	return &Handlers{provider: p, sessions: sm, users: us, adminEmail: adminEmail}
}

// Login initiates the OIDC authorization code flow with PKCE.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	state, err := GenerateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Store state and verifier in short-lived cookies
	setPreAuthCookie(w, cookieState, state)
	setPreAuthCookie(w, cookieCodeVerifier, verifier)

	// Preserve the redirect URL
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/dashboard"
	}
	setPreAuthCookie(w, cookieRedirect, redirect)

	http.Redirect(w, r, h.provider.AuthCodeURL(state, challenge), http.StatusFound)
}

// Callback handles the OIDC provider redirect after authentication.
func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	// Validate state
	stateCookie, err := r.Cookie(cookieState)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Get PKCE verifier
	verifierCookie, err := r.Cookie(cookieCodeVerifier)
	if err != nil {
		http.Error(w, "missing code verifier", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	idToken, err := h.provider.Exchange(r.Context(), r.URL.Query().Get("code"), verifierCookie.Value)
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	// Extract claims
	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "invalid claims", http.StatusUnauthorized)
		return
	}

	// Upsert user record
	user, err := h.users.Upsert(r.Context(), idToken.Issuer, claims.Subject, claims.Email, claims.Name, h.adminEmail)
	if err != nil {
		http.Error(w, "user record error", http.StatusInternalServerError)
		return
	}

	// Create session
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	h.sessions.Put(r.Context(), SessionUserIDKey, user.ID)
	h.sessions.Put(r.Context(), SessionRoleKey, user.Role)

	// Clear pre-auth cookies
	clearCookie(w, cookieState)
	clearCookie(w, cookieCodeVerifier)

	// Redirect
	redirectCookie, err := r.Cookie(cookieRedirect)
	redirect := "/dashboard"
	if err == nil && redirectCookie.Value != "" {
		redirect = redirectCookie.Value
	}
	clearCookie(w, cookieRedirect)

	http.Redirect(w, r, redirect, http.StatusFound)
}

// Logout destroys the session and redirects to the login page.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Destroy(r.Context()); err != nil {
		http.Error(w, "logout error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

func setPreAuthCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
}
