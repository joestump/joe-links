// Governing: SPEC-0004 REQ "Landing Page", ADR-0007
package handler

import (
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
)

// LandingHandler serves the public landing page.
type LandingHandler struct{}

// NewLandingHandler creates a new LandingHandler.
func NewLandingHandler() *LandingHandler { return &LandingHandler{} }

// Index serves GET /. Authenticated users are redirected to /dashboard.
func (h *LandingHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	render(w, "landing.html", newBasePage(r, nil))
}
