// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/store"
)

// registerKeywordRoutes registers the public /keywords endpoint.
// No auth required -- the browser extension calls this without a token.
// Governing: SPEC-0008 REQ "Keyword Host Discovery", ADR-0011
func registerKeywordRoutes(r chi.Router, keywords *store.KeywordStore) {
	r.Get("/keywords", func(w http.ResponseWriter, r *http.Request) {
		list, err := keywords.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list keywords", "INTERNAL_ERROR")
			return
		}
		names := make([]string, len(list))
		for i, k := range list {
			names[i] = k.Keyword
		}
		writeJSON(w, http.StatusOK, names)
	})
}

// KeywordTemplateResponse is the shape returned by GET /api/v1/keywords/templates.
type KeywordTemplateResponse struct {
	Keyword     string `json:"keyword"`
	URLTemplate string `json:"url_template"`
}

// registerKeywordTemplateRoutes registers the authenticated /keywords/templates endpoint.
// Returns full keyword + url_template data so the browser extension can match templates
// against the current tab URL and suggest keyword-based shortcuts.
func registerKeywordTemplateRoutes(r chi.Router, keywords *store.KeywordStore) {
	r.Get("/keywords/templates", func(w http.ResponseWriter, r *http.Request) {
		list, err := keywords.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list keyword templates", "INTERNAL_ERROR")
			return
		}
		resp := make([]KeywordTemplateResponse, len(list))
		for i, k := range list {
			resp[i] = KeywordTemplateResponse{Keyword: k.Keyword, URLTemplate: k.URLTemplate}
		}
		writeJSON(w, http.StatusOK, resp)
	})
}
