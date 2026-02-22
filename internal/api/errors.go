// Governing: SPEC-0005 REQ "Standard Error Response Format", ADR-0008
package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError writes a JSON error response with the given HTTP status code.
// Governing: SPEC-0005 REQ "Standard Error Response Format"
func writeError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorBody{Error: message, Code: code})
}

// isDBLockError reports whether err is a database locking/busy error.
// Covers SQLite "database is locked", MySQL "deadlock", PostgreSQL serialization failures.
func isDBLockError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "deadlock") ||
		strings.Contains(msg, "could not serialize")
}

// writeJSON writes a JSON response with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
