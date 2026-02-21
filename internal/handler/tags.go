// Governing: SPEC-0004 REQ "Tag Browser", "New Link Form", ADR-0007
package handler

import (
	"html"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// TagsHandler serves tag browsing views.
type TagsHandler struct {
	tags  *store.TagStore
	links *store.LinkStore
}

// NewTagsHandler creates a new TagsHandler.
func NewTagsHandler(ts *store.TagStore, ls *store.LinkStore) *TagsHandler {
	return &TagsHandler{tags: ts, links: ls}
}

// TagIndexPage is the template data for the tag browser.
type TagIndexPage struct {
	BasePage
	Tags []*store.TagWithCount
}

// TagDetailPage is the template data for the tag detail view.
type TagDetailPage struct {
	BasePage
	Tag   *store.Tag
	Links []*store.Link
}

// Index renders all tags with ≥1 link and their counts.
// Governing: SPEC-0004 REQ "Tag Browser"
func (h *TagsHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	tags, _ := h.tags.ListWithCounts(r.Context())
	data := TagIndexPage{
		BasePage: BasePage{Theme: themeFromRequest(r), User: user},
		Tags:     tags,
	}
	if isHTMX(r) {
		renderFragment(w, "content", data)
		return
	}
	render(w, "tags/index.html", data)
}

// Detail renders links for a specific tag.
// Governing: SPEC-0004 REQ "Tag Browser"
func (h *TagsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	slug := chi.URLParam(r, "slug")
	tag, err := h.tags.GetBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	links, _ := h.links.ListByTag(r.Context(), slug)
	data := TagDetailPage{
		BasePage: BasePage{Theme: themeFromRequest(r), User: user},
		Tag:      tag,
		Links:    links,
	}
	if isHTMX(r) {
		renderFragment(w, "content", data)
		return
	}
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
