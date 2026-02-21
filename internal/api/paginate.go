// Governing: SPEC-0005 REQ "Pagination", ADR-0008
package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// parsePagination extracts cursor and limit from query parameters.
// limit defaults to 50 and is silently capped at 200.
// Governing: SPEC-0005 REQ "Pagination"
func parsePagination(r *http.Request) (cursor string, limit int) {
	cursor = r.URL.Query().Get("cursor")
	limit = defaultLimit

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	return cursor, limit
}

// encodeCursor encodes an opaque pagination cursor from a string value (typically
// the last item's sort key, e.g. slug or created_at).
func encodeCursor(value string) string {
	return base64.URLEncoding.EncodeToString([]byte(value))
}

// decodeCursor decodes an opaque pagination cursor back to the original string.
// Returns an empty string if the cursor is empty or invalid.
func decodeCursor(cursor string) string {
	if cursor == "" {
		return ""
	}
	b, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return ""
	}
	return string(b)
}
