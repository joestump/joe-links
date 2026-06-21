// Governing: SPEC-0005 REQ "Pagination", ADR-0008
package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
)

// Pagination defaults per SPEC-0005 REQ "Pagination".
const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

// parseLimit reads ?limit=N (default 50, max 200). Invalid, zero, or negative
// values fall back to the default; values above the max are silently capped.
// Governing: SPEC-0005 REQ "Pagination" — Scenario Default Limit Applied, Scenario Limit Capped
func parseLimit(r *http.Request) int {
	limit := defaultPageLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	return limit
}

// parseCursor reads ?cursor= and decodes the opaque keyset cursor into its
// (sortKey, id) components. Returns empty strings when no cursor is present or
// the value is malformed (malformed cursors restart from the first page).
// Governing: SPEC-0005 REQ "Pagination"
func parseCursor(r *http.Request) (sortKey, id string) {
	return decodeCursor(r.URL.Query().Get("cursor"))
}

// encodeCursor builds an opaque base64 cursor from a row's sort key and id.
// The cursor encodes "sortKey\x00id" so keyset pagination has a deterministic
// tiebreaker when multiple rows share the same sort key.
// Governing: SPEC-0005 REQ "Pagination"
func encodeCursor(sortKey, id string) string {
	return base64.URLEncoding.EncodeToString([]byte(sortKey + "\x00" + id))
}

// decodeCursor reverses encodeCursor. Malformed input yields empty components.
// Governing: SPEC-0005 REQ "Pagination"
func decodeCursor(cursor string) (sortKey, id string) {
	if cursor == "" {
		return "", ""
	}
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
