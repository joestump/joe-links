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

func render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
