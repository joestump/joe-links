// Governing: SPEC-0005 REQ "Standard Error Response Format", ADR-0008
package api

import (
	"encoding/json"
	"net/http"
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

// writeJSON writes a JSON response with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
