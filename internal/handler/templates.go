// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions", REQ "DaisyUI and Tailwind CSS", ADR-0001
// Governing: SPEC-0003 REQ "System-Preference Default", SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/web"
)

// BasePage carries layout-level data available to every template.
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie"
// Governing: SPEC-0004 REQ "Shared Base Layout" — User enables conditional admin nav link
type BasePage struct {
	Theme string      // "joe-light", "joe-dark", or "" (let inline script decide)
	User  *store.User // nil for unauthenticated pages
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

// pageCache maps a render key (e.g. "dashboard.html", "tags/index.html") to a
// compiled template set containing base.html + partials + that one page file.
// Each page gets its own set so {{define "content"}} blocks don't collide.
var (
	pageCache    map[string]*template.Template
	fragmentTmpl *template.Template
)

func init() {
	partials, err := fs.Glob(web.TemplateFS, "templates/partials/*.html")
	if err != nil {
		panic("glob partials: " + err.Error())
	}

	// Standalone set for global HTMX fragment rendering (partials only).
	fragmentTmpl = template.Must(template.New("").ParseFS(web.TemplateFS, partials...))

	// Count how many page files share each basename to detect collisions.
	baseCount := map[string]int{}
	_ = fs.WalkDir(web.TemplateFS, "templates/pages", func(p string, d fs.DirEntry, e error) error {
		if e != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return e
		}
		baseCount[filepath.Base(p)]++
		return nil
	})

	// Build one template set per page file.
	pageCache = make(map[string]*template.Template)
	err = fs.WalkDir(web.TemplateFS, "templates/pages", func(p string, d fs.DirEntry, e error) error {
		if e != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return e
		}

		files := make([]string, 0, 2+len(partials))
		files = append(files, "templates/base.html")
		files = append(files, partials...)
		files = append(files, p)

		t, err := template.New("").ParseFS(web.TemplateFS, files...)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}

		// Primary key: path relative to "templates/pages/" (always unambiguous).
		rel, _ := strings.CutPrefix(p, "templates/pages/")
		pageCache[rel] = t

		// Alias under bare basename when it is unique across all page files.
		base := filepath.Base(p)
		if baseCount[base] == 1 {
			pageCache[base] = t
		}

		return nil
	})
	if err != nil {
		panic("build page cache: " + err.Error())
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

// render executes a full-page template (base layout + named page).
// tmpl is the render key, e.g. "dashboard.html" or "tags/index.html".
func render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := pageCache[tmpl]
	if !ok {
		http.Error(w, "template not found: "+tmpl, http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// renderFragment executes a named template from the global partials set.
// Use for standalone HTMX partials (link_list, token_list, owners_list, etc.).
func renderFragment(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := fragmentTmpl.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// renderPageFragment executes a named template from a specific page's template set.
// Use for HTMX partial renders that need a page-specific block (e.g. "content")
// or a page-local named template (e.g. "user_row" in admin/users.html).
func renderPageFragment(w http.ResponseWriter, page, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := pageCache[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
