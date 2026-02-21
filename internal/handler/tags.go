// Governing: SPEC-0004 REQ "Tag Browser", ADR-0007
package handler

import (
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

// Suggest returns tag autocomplete results.
func (h *TagsHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(""))
}
