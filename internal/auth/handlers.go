// Governing: SPEC-0001 REQ "OIDC-Only Authentication", ADR-0003
package auth

import (
	"log"
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
	provider      *Provider
	sessions      *scs.SessionManager
	users         *store.UserStore
	adminEmail    string
	adminGroups   []string // OIDC group names that grant the admin role
	groupsClaim   string   // OIDC claim name for groups (default: "groups")
	secureCookies bool
}

// NewHandlers creates a new Handlers with the given dependencies.
// Set secureCookies=false for local HTTP development.
func NewHandlers(p *Provider, sm *scs.SessionManager, us *store.UserStore, adminEmail string, adminGroups []string, groupsClaim string, secureCookies bool) *Handlers {
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	return &Handlers{
		provider:      p,
		sessions:      sm,
		users:         us,
		adminEmail:    adminEmail,
		adminGroups:   adminGroups,
		groupsClaim:   groupsClaim,
		secureCookies: secureCookies,
	}
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
	h.setPreAuthCookie(w, cookieState, state)
	h.setPreAuthCookie(w, cookieCodeVerifier, verifier)

	// Preserve the redirect URL
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/dashboard"
	}
	h.setPreAuthCookie(w, cookieRedirect, redirect)

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

	// Extract claims — groups claim is dynamic based on config.
	var rawClaims map[string]interface{}
	if err := idToken.Claims(&rawClaims); err != nil {
		http.Error(w, "invalid claims", http.StatusUnauthorized)
		return
	}
	email, _ := rawClaims["email"].(string)
	name, _ := rawClaims["name"].(string)
	subject, _ := rawClaims["sub"].(string)

	// Determine role from adminEmail and OIDC group membership.
	role := "user"
	if h.adminEmail != "" && email == h.adminEmail {
		role = "admin"
	}
	if role != "admin" && len(h.adminGroups) > 0 {
		if groups := rawClaims[h.groupsClaim]; groups != nil {
			var userGroups []string
			switch v := groups.(type) {
			case []interface{}:
				for _, g := range v {
					if s, ok := g.(string); ok {
						userGroups = append(userGroups, s)
					}
				}
			case []string:
				userGroups = v
			}
			adminSet := make(map[string]struct{}, len(h.adminGroups))
			for _, g := range h.adminGroups {
				adminSet[g] = struct{}{}
			}
			for _, g := range userGroups {
				if _, ok := adminSet[g]; ok {
					role = "admin"
					break
				}
			}
		}
	}

	// Upsert user record — role is enforced on every login.
	user, err := h.users.Upsert(r.Context(), idToken.Issuer, subject, email, name, role)
	if err != nil {
		log.Printf("auth callback: upsert user (issuer=%s subject=%s email=%s): %v", idToken.Issuer, subject, email, err)
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

func (h *Handlers) setPreAuthCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   h.secureCookies,
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
