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
