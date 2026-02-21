// Governing: SPEC-0003 REQ "HTMX Theme Endpoint", SPEC-0003 REQ "Theme Toggle Control",
// SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
package handler

import (
	"encoding/json"
	"net/http"
)

// ThemeHandler handles the theme toggle endpoint.
type ThemeHandler struct{}

// NewThemeHandler creates a new ThemeHandler.
func NewThemeHandler() *ThemeHandler {
	return &ThemeHandler{}
}

// Toggle handles POST /dashboard/theme.
// No auth required — sets the theme cookie and returns HX-Trigger for client-side swap.
// Governing: SPEC-0003 REQ "HTMX Theme Endpoint"
func (h *ThemeHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	theme := r.FormValue("theme")
	if theme != "joe-light" && theme != "joe-dark" {
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}

	// Persist via cookie (non-HttpOnly so the anti-flash script can read it).
	// Governing: SPEC-0003 REQ "Theme Persistence via Cookie"
	http.SetCookie(w, &http.Cookie{
		Name:     "theme",
		Value:    theme,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		SameSite: http.SameSiteLaxMode,
		HttpOnly: false,
	})

	// Return HX-Trigger for client-side data-theme swap.
	// Governing: SPEC-0003 REQ "Theme Toggle Control"
	trigger, _ := json.Marshal(map[string]any{
		"themeChanged": map[string]string{"theme": theme},
	})
	w.Header().Set("HX-Trigger", string(trigger))
	w.WriteHeader(http.StatusOK)
}
