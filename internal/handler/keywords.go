// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
package handler

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

var keywordRE = regexp.MustCompile(`^[a-z][a-z0-9\-]*$`)

// KeywordsHandler serves admin keyword CRUD views.
type KeywordsHandler struct {
	keywords *store.KeywordStore
}

// NewKeywordsHandler creates a new KeywordsHandler.
func NewKeywordsHandler(ks *store.KeywordStore) *KeywordsHandler {
	return &KeywordsHandler{keywords: ks}
}

// AdminKeywordsPage is the template data for the keywords list.
type AdminKeywordsPage struct {
	BasePage
	Keywords []*store.Keyword
	Error    string
}

// Index renders the keyword management list.
// GET /admin/keywords
// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
func (h *KeywordsHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	keywords, _ := h.keywords.List(r.Context())
	data := AdminKeywordsPage{
		BasePage: BasePage{Theme: themeFromRequest(r), User: user},
		Keywords: keywords,
	}
	render(w, "admin/keywords.html", data)
}

// Create processes the inline keyword creation form.
// POST /admin/keywords
// Validates: keyword non-empty, lowercase alphanumeric+hyphens ([a-z][a-z0-9-]*)
// Validates: url_template contains "{slug}"
// On error: re-render keyword_list partial with error via HTMX
// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
func (h *KeywordsHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	keyword := strings.TrimSpace(r.FormValue("keyword"))
	urlTemplate := strings.TrimSpace(r.FormValue("url_template"))
	description := strings.TrimSpace(r.FormValue("description"))

	if keyword == "" || urlTemplate == "" {
		h.renderList(w, r, user, "Keyword and URL template are required.")
		return
	}

	if !keywordRE.MatchString(keyword) {
		h.renderList(w, r, user, "Keyword must be lowercase letters, digits, and hyphens (e.g. jira, my-tool).")
		return
	}

	if !strings.Contains(urlTemplate, "{slug}") {
		h.renderList(w, r, user, "URL template must contain {slug} placeholder.")
		return
	}

	_, err := h.keywords.Create(r.Context(), keyword, urlTemplate, description)
	if err != nil {
		h.renderList(w, r, user, "A keyword with that name already exists.")
		return
	}

	h.renderList(w, r, user, "")
}

// Delete removes a keyword. Returns empty 200 so HTMX swaps out the row.
// DELETE /admin/keywords/{id}
// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
func (h *KeywordsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.keywords.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// renderList re-renders the keyword_list partial (or full page for non-HTMX).
func (h *KeywordsHandler) renderList(w http.ResponseWriter, r *http.Request, user *store.User, errMsg string) {
	keywords, _ := h.keywords.List(r.Context())
	data := AdminKeywordsPage{
		BasePage: BasePage{Theme: themeFromRequest(r), User: user},
		Keywords: keywords,
		Error:    errMsg,
	}
	if isHTMX(r) {
		renderPageFragment(w, "admin/keywords.html", "keyword_list", data)
		return
	}
	render(w, "admin/keywords.html", data)
}
