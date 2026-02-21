// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions", REQ "DaisyUI and Tailwind CSS", ADR-0001
package handler

import (
	"html/template"
	"net/http"

	"github.com/joestump/joe-links/web"
)

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
