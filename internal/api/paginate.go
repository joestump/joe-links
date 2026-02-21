// Governing: SPEC-0005 REQ "Pagination", ADR-0008
package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
)

const defaultLimit = 50
const maxLimit = 200

// parsePagination extracts limit and cursor from query parameters.
// limit defaults to 50 and is silently capped at 200.
// Governing: SPEC-0005 REQ "Pagination"
func parsePagination(r *http.Request) (limit int, cursor string) {
	limit = defaultLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	cursor = r.URL.Query().Get("cursor")
	return
}

// encodeCursor encodes an opaque pagination cursor from a string value (typically
// the last item's sort key, e.g. slug or created_at).
func encodeCursor(val string) string {
	return base64.URLEncoding.EncodeToString([]byte(val))
}

// decodeCursor decodes an opaque pagination cursor back to the original string.
// Returns an empty string if the cursor is empty or invalid.
func decodeCursor(encoded string) string {
	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(b)
}
