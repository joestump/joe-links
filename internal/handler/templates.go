// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions", REQ "DaisyUI and Tailwind CSS", ADR-0001
// Governing: SPEC-0003 REQ "System-Preference Default", SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
package handler

import (
	"html/template"
	"net/http"

	"github.com/joestump/joe-links/web"
)

// BasePage carries layout-level data available to every template.
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie"
type BasePage struct {
	Theme string // "joe-light", "joe-dark", or "" (let inline script decide)
}

// themeFromRequest reads the "theme" cookie. Returns "" if absent or invalid,
// so the server omits data-theme and lets the anti-flash inline script handle it.
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie"
func themeFromRequest(r *http.Request) string {
	c, err := r.Cookie("theme")
	if err != nil {
		return ""
	}
	if c.Value == "joe-light" || c.Value == "joe-dark" {
		return c.Value
	}
	return ""
}

var templates *template.Template

func init() {
	var err error
	templates, err = template.New("").ParseFS(web.TemplateFS,
		"templates/base.html",
		"templates/partials/*.html",
		"templates/pages/*.html",
		"templates/pages/links/*.html",
	)
	if err != nil {
		panic("failed to parse templates: " + err.Error())
	}
}

// Flash represents a one-time notification message shown to the user.
type Flash struct {
	Type    string // "success", "error", "warning", "info"
	Message string
}

// isHTMX returns true when the request was sent by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// render executes a full-page template (base layout + named page block).
func render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// renderFragment executes a named template block without the base layout,
// returning only the HTML fragment for HTMX target swapping.
func renderFragment(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
