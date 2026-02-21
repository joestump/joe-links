// Governing: SPEC-0004 REQ "Tag Browser", "New Link Form", ADR-0007
package handler

import (
	"html"
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// TagsHandler serves tag browsing views.
type TagsHandler struct {
	tags *store.TagStore
}

// NewTagsHandler creates a new TagsHandler.
func NewTagsHandler(ts *store.TagStore) *TagsHandler { return &TagsHandler{tags: ts} }

// Index renders all tags with counts.
func (h *TagsHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := BasePage{Theme: themeFromRequest(r), User: user}
	render(w, "tags/index.html", data)
}

// Detail renders links for a specific tag.
func (h *TagsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := BasePage{Theme: themeFromRequest(r), User: user}
	render(w, "tags/detail.html", data)
}

// Suggest returns tag autocomplete results as HTML options.
// Governing: SPEC-0004 REQ "New Link Form" — tag autocomplete via HTMX
func (h *TagsHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	w.Header().Set("Content-Type", "text/html")
	if q == "" {
		w.Write([]byte(""))
		return
	}
	tags, err := h.tags.SearchByPrefix(r.Context(), q)
	if err != nil || len(tags) == 0 {
		w.Write([]byte(""))
		return
	}
	var buf []byte
	for _, t := range tags {
		buf = append(buf, []byte(`<li><button type="button" class="btn btn-ghost btn-sm justify-start" onclick="addTag('`+html.EscapeString(t.Name)+`')">`+html.EscapeString(t.Name)+`</button></li>`)...)
	}
	w.Write(buf)
}
